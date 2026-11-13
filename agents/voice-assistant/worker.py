#!/usr/bin/env python3
"""Consume SuIM rtc agent dispatch jobs and run LiveKit Agents AgentSession."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import signal
import time
from typing import Any

import jwt
import redis.asyncio as redis
from livekit import rtc

from playback_tracker import TOPIC as PLAYBACK_TOPIC
from playback_tracker import PlaybackTracker
from voice_pipeline import has_llm, run_agent_session

LOG = logging.getLogger("suim-agent")

STREAM = os.environ.get("AGENT_STREAM", "suim:agent:dispatch")
GROUP = os.environ.get("AGENT_GROUP", "suim-agent-workers")
CONSUMER = os.environ.get("AGENT_CONSUMER", f"worker-{os.getpid()}")
REDIS_URL = os.environ.get("REDIS_URL", "redis://127.0.0.1:6379/0")
LIVEKIT_URL = os.environ.get("LIVEKIT_URL", "ws://127.0.0.1:7880")
LIVEKIT_API_KEY = os.environ.get("LIVEKIT_API_KEY", "suim")
LIVEKIT_API_SECRET = os.environ.get("LIVEKIT_API_SECRET", "")


def access_token(identity: str, room: str, ttl_sec: int = 600) -> str:
    now = int(time.time())
    claims = {
        "iss": LIVEKIT_API_KEY,
        "sub": identity,
        "nbf": now - 10,
        "exp": now + ttl_sec,
        "video": {
            "roomJoin": True,
            "room": room,
            "canPublish": True,
            "canSubscribe": True,
            "canPublishSources": ["microphone"],
        },
    }
    return jwt.encode(claims, LIVEKIT_API_SECRET, algorithm="HS256")


async def ensure_group(rdb: redis.Redis) -> None:
    try:
        await rdb.xgroup_create(STREAM, GROUP, id="$", mkstream=True)
        LOG.info("created consumer group %s on %s", GROUP, STREAM)
    except redis.ResponseError as err:
        if "BUSYGROUP" not in str(err):
            raise


async def publish_text(room: rtc.Room, topic: str, text: str) -> None:
    try:
        await room.local_participant.publish_data(
            text.encode("utf-8"),
            reliable=True,
            topic=topic,
        )
    except Exception as err:  # noqa: BLE001
        LOG.warning("publish_data %s failed: %s", topic, err)


async def run_session(job: dict[str, Any]) -> None:
    room_id = job["room_id"]
    identity = job.get("identity") or "agent:assistant"
    linked = job.get("linked_user_id") or job.get("caller_id") or ""
    LOG.info(
        "joining room=%s identity=%s linked=%s reason=%s llm=%s",
        room_id,
        identity,
        linked,
        job.get("reason"),
        has_llm(),
    )
    if not LIVEKIT_API_SECRET:
        raise RuntimeError("LIVEKIT_API_SECRET is required")
    if not has_llm():
        raise RuntimeError("OPENAI_API_KEY is required for AgentSession voice pipeline")

    room = rtc.Room()
    left = asyncio.Event()
    tracker = PlaybackTracker()

    @room.on("disconnected")
    def _on_disconnected(*_args: Any) -> None:
        LOG.info("room disconnected room=%s", room_id)
        left.set()

    @room.on("participant_disconnected")
    def _on_participant_left(participant: rtc.RemoteParticipant) -> None:
        if linked and participant.identity == linked:
            LOG.info("linked user left room=%s", room_id)
            left.set()
        elif len(room.remote_participants) == 0:
            left.set()

    @room.on("data_received")
    def _on_data(data: rtc.DataPacket) -> None:
        topic = getattr(data, "topic", None) or ""
        if topic != PLAYBACK_TOPIC:
            return
        payload = data.data
        tracker.ingest_client_event(payload)

    token = access_token(identity, room_id)
    await room.connect(LIVEKIT_URL, token)
    LOG.info("connected room=%s participants=%d", room_id, len(room.remote_participants))

    async def on_transcript(role: str, text: str) -> None:
        await publish_text(
            room,
            "suim.agent.transcript",
            json.dumps({"role": role, "text": text}, ensure_ascii=False),
        )

    async def on_publish(topic: str, text: str) -> None:
        await publish_text(room, topic, text)

    tracker.bind_publish(on_publish)

    session_task = asyncio.create_task(
        run_agent_session(
            room,
            linked_user_id=linked,
            on_transcript=on_transcript,
            on_publish=on_publish,
            playback=tracker,
        ),
        name="agent-session",
    )
    wait_left = asyncio.create_task(left.wait(), name="room-left")
    try:
        finished, pending = await asyncio.wait(
            {session_task, wait_left},
            timeout=60 * 30,
            return_when=asyncio.FIRST_COMPLETED,
        )
        for task in pending:
            task.cancel()
        for task in finished:
            if task is session_task and not task.cancelled():
                exc = task.exception()
                if exc:
                    raise exc
    finally:
        if not session_task.done():
            session_task.cancel()
            try:
                await session_task
            except (asyncio.CancelledError, Exception):  # noqa: BLE001
                pass
        await room.disconnect()
        LOG.info("left room=%s", room_id)


async def handle_message(rdb: redis.Redis, msg_id: str, fields: dict[str, Any]) -> None:
    raw = fields.get("body") or fields.get(b"body")
    if isinstance(raw, bytes):
        raw = raw.decode("utf-8")
    if not raw:
        await rdb.xack(STREAM, GROUP, msg_id)
        return
    job = json.loads(raw)
    try:
        await run_session(job)
    finally:
        await rdb.xack(STREAM, GROUP, msg_id)


async def consume() -> None:
    if not LIVEKIT_API_SECRET:
        raise SystemExit("set LIVEKIT_API_SECRET to the LiveKit signing secret")
    if not has_llm():
        raise SystemExit("set OPENAI_API_KEY (SiliconFlow / OpenAI-compatible) for AgentSession")

    heartbeat = 0
    while True:
        rdb = redis.from_url(
            REDIS_URL,
            decode_responses=True,
            socket_connect_timeout=5,
            socket_timeout=None,
        )
        try:
            await rdb.ping()
            await ensure_group(rdb)
            LOG.info(
                "listening stream=%s group=%s consumer=%s llm_configured=%s",
                STREAM,
                GROUP,
                CONSUMER,
                has_llm(),
            )
            while True:
                try:
                    rows = await rdb.xreadgroup(
                        GROUP, CONSUMER, streams={STREAM: ">"}, count=1, block=2000
                    )
                except (TimeoutError, redis.TimeoutError, asyncio.TimeoutError):
                    rows = []
                except Exception:  # noqa: BLE001
                    LOG.exception("xreadgroup failed; reconnecting")
                    break
                heartbeat += 1
                if heartbeat % 30 == 0:
                    LOG.info("heartbeat consumer=%s", CONSUMER)
                if not rows:
                    continue
                for _stream, messages in rows:
                    for msg_id, fields in messages:
                        LOG.info("got job id=%s", msg_id)
                        try:
                            await handle_message(rdb, msg_id, fields)
                        except Exception:  # noqa: BLE001
                            LOG.exception("job failed id=%s", msg_id)
                            try:
                                await rdb.xack(STREAM, GROUP, msg_id)
                            except Exception:  # noqa: BLE001
                                LOG.exception("xack failed id=%s", msg_id)
        finally:
            try:
                await rdb.aclose()
            except Exception:  # noqa: BLE001
                pass
        await asyncio.sleep(1)


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    stop = loop.create_future()

    def _stop(*_args: Any) -> None:
        if not stop.done():
            stop.set_result(True)

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _stop)
        except NotImplementedError:
            signal.signal(sig, lambda *_: _stop())

    async def _run() -> None:
        task = asyncio.create_task(consume())
        await stop
        task.cancel()
        try:
            await task
        except asyncio.CancelledError:
            pass

    loop.run_until_complete(_run())


if __name__ == "__main__":
    main()
