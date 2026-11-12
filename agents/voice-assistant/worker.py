#!/usr/bin/env python3
"""Consume SuIM rtc agent dispatch jobs and join LiveKit rooms.

Env:
  LIVEKIT_URL          ws://127.0.0.1:7880
  LIVEKIT_API_KEY      suim
  LIVEKIT_API_SECRET   (same as rtc.yaml livekit.secret)
  REDIS_URL            redis://:password@127.0.0.1:6379/0
  AGENT_STREAM         suim:agent:dispatch
  AGENT_GROUP          suim-agent-workers
  AGENT_GREETING       optional spoken/text greeting after join

Phase 1 joins the room as identity from the job and stays until the room
is empty or the linked user disconnects. Wire STT/LLM/TTS (LiveKit Agents)
in a later phase when model API keys are available.
"""

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

LOG = logging.getLogger("suim-agent")

STREAM = os.environ.get("AGENT_STREAM", "suim:agent:dispatch")
GROUP = os.environ.get("AGENT_GROUP", "suim-agent-workers")
CONSUMER = os.environ.get("AGENT_CONSUMER", f"worker-{os.getpid()}")
REDIS_URL = os.environ.get("REDIS_URL", "redis://127.0.0.1:6379/0")
LIVEKIT_URL = os.environ.get("LIVEKIT_URL", "ws://127.0.0.1:7880")
LIVEKIT_API_KEY = os.environ.get("LIVEKIT_API_KEY", "suim")
LIVEKIT_API_SECRET = os.environ.get("LIVEKIT_API_SECRET", "")
GREETING = os.environ.get("AGENT_GREETING", "你好，我是 SuIM 语音助手。")


def access_token(identity: str, room: str, ttl_sec: int = 600) -> str:
    now = int(time.time())
    claims = {
        "iss": LIVEKIT_API_KEY,
        "sub": identity,
        "nbf": now - 10,
        "exp": now + ttl_sec,
        # Match SuIM pkg/livekit token grant shape (microphone publish + subscribe).
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


async def run_session(job: dict[str, Any]) -> None:
    room_id = job["room_id"]
    identity = job.get("identity") or "agent:assistant"
    linked = job.get("linked_user_id") or ""
    LOG.info(
        "joining room=%s identity=%s linked=%s reason=%s",
        room_id,
        identity,
        linked,
        job.get("reason"),
    )
    if not LIVEKIT_API_SECRET:
        raise RuntimeError("LIVEKIT_API_SECRET is required")

    room = rtc.Room()
    done = asyncio.Event()

    @room.on("disconnected")
    def _on_disconnected(*_args: Any) -> None:
        done.set()

    @room.on("participant_disconnected")
    def _on_participant_left(participant: rtc.RemoteParticipant) -> None:
        if linked and participant.identity == linked:
            LOG.info("linked user left room=%s", room_id)
            done.set()
        elif len(room.remote_participants) == 0:
            done.set()

    token = access_token(identity, room_id)
    await room.connect(LIVEKIT_URL, token)
    LOG.info("connected room=%s participants=%d", room_id, len(room.remote_participants))

    try:
        await room.local_participant.publish_data(
            GREETING.encode("utf-8"),
            reliable=True,
            topic="suim.agent.greeting",
        )
    except Exception as err:  # noqa: BLE001
        LOG.warning("greeting data failed: %s", err)

    try:
        await asyncio.wait_for(done.wait(), timeout=60 * 30)
    except asyncio.TimeoutError:
        LOG.info("session timeout room=%s", room_id)
    finally:
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
    rdb = redis.from_url(REDIS_URL, decode_responses=True)
    await ensure_group(rdb)
    LOG.info("listening stream=%s group=%s consumer=%s", STREAM, GROUP, CONSUMER)
    while True:
        rows = await rdb.xreadgroup(GROUP, CONSUMER, streams={STREAM: ">"}, count=1, block=5000)
        if not rows:
            continue
        for _stream, messages in rows:
            for msg_id, fields in messages:
                try:
                    await handle_message(rdb, msg_id, fields)
                except Exception:  # noqa: BLE001
                    LOG.exception("job failed id=%s", msg_id)
                    await rdb.xack(STREAM, GROUP, msg_id)


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
