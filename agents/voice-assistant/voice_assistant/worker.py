#!/usr/bin/env python3
"""Consume rtc AgentJob from Redis and join the LiveKit room.

P0: join + SiliconFlow greeting (LLM text + TTS PCM) when API key is set.
Next: duplex ASR→LLM→TTS on the user audio track.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import signal
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import jwt
import redis.asyncio as redis
from livekit import rtc

from voice_assistant.config import RuntimeConfig, load_dotenv
from voice_assistant.models.base import ChatMessage
from voice_assistant.models.factory import ModelBundle, build_models

log = logging.getLogger("suim.agent")


@dataclass
class AgentJob:
    room_id: str
    linked_user_id: str
    identity: str = "agent:assistant"
    caller_id: str = ""
    callee_id: str = ""
    media: str = "audio"
    kind: str = "single"
    reason: str = ""
    voice_id: str = ""

    @classmethod
    def from_body(cls, raw: str | bytes) -> AgentJob:
        data = json.loads(raw)
        return cls(
            room_id=str(data.get("room_id") or ""),
            linked_user_id=str(data.get("linked_user_id") or ""),
            identity=str(data.get("identity") or "agent:assistant"),
            caller_id=str(data.get("caller_id") or ""),
            callee_id=str(data.get("callee_id") or ""),
            media=str(data.get("media") or "audio"),
            kind=str(data.get("kind") or "single"),
            reason=str(data.get("reason") or ""),
            voice_id=str(data.get("voice_id") or ""),
        )


def livekit_token(cfg: RuntimeConfig, room: str, identity: str) -> str:
    """HS256 token compatible with SuIM pkg/livekit SignCall grants."""
    now = int(time.time())
    payload = {
        "iss": cfg.livekit_api_key,
        "sub": identity,
        "nbf": now - 10,
        "exp": now + 6 * 3600,
        "video": {
            "roomJoin": True,
            "room": room,
            "canPublish": True,
            "canSubscribe": True,
            "canPublishData": True,
            "canPublishSources": ["microphone"],
        },
        "metadata": json.dumps({"role": "agent"}),
    }
    return jwt.encode(payload, cfg.livekit_api_secret, algorithm="HS256")


async def publish_data(room: rtc.Room, topic: str, payload: dict[str, Any]) -> None:
    await room.local_participant.publish_data(
        json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        reliable=True,
        topic=topic,
    )


async def publish_pcm16(room: rtc.Room, pcm: bytes, sample_rate: int = 24000) -> None:
    if not pcm:
        return
    source = rtc.AudioSource(sample_rate, 1)
    track = rtc.LocalAudioTrack.create_audio_track("agent-voice", source)
    await room.local_participant.publish_track(track)
    samples_per_frame = max(sample_rate // 50, 1)
    frame_bytes = samples_per_frame * 2
    for i in range(0, len(pcm), frame_bytes):
        chunk = pcm[i : i + frame_bytes]
        if len(chunk) < frame_bytes:
            chunk = chunk + b"\x00" * (frame_bytes - len(chunk))
        frame = rtc.AudioFrame(chunk, sample_rate, 1, samples_per_frame)
        await source.capture_frame(frame)
        await asyncio.sleep(0.02)


async def handle_job(cfg: RuntimeConfig, models: ModelBundle | None, job: AgentJob) -> None:
    if not job.room_id or not job.linked_user_id:
        log.warning("skip invalid job: %s", job)
        return
    if not cfg.livekit_api_secret:
        raise RuntimeError("LIVEKIT_API_SECRET is required")

    room = rtc.Room()
    token = livekit_token(cfg, job.room_id, job.identity)
    log.info("joining room=%s identity=%s reason=%s", job.room_id, job.identity, job.reason)
    await room.connect(cfg.livekit_url, token)

    try:
        await publish_data(room, "suim.agent.state", {"type": "suim.agent.state", "state": "listening"})

        greeting = "你好，我是苏IM语音助手，有什么可以帮你？"
        if models is not None:
            try:
                greeting = (
                    await models.llm.chat(
                        [
                            ChatMessage(
                                role="system",
                                content="你是苏IM中文语音助手，用一句简短问候开场，不要超过20字。",
                            ),
                            ChatMessage(role="user", content="请打个招呼。"),
                        ],
                        temperature=0.4,
                    )
                    or greeting
                )
            except Exception:
                log.exception("llm greeting failed; using default")

            await publish_data(
                room,
                "suim.agent.transcript",
                {"type": "suim.agent.transcript", "role": "assistant", "text": greeting, "final": True},
            )
            try:
                await publish_data(room, "suim.agent.state", {"type": "suim.agent.state", "state": "speaking"})
                audio = await models.tts.synthesize(
                    greeting,
                    voice=job.voice_id or None,
                    response_format="pcm",
                    sample_rate=24000,
                )
                await publish_pcm16(room, audio.audio, sample_rate=audio.sample_rate or 24000)
            except Exception:
                log.exception("tts greeting failed")
        else:
            await publish_data(
                room,
                "suim.agent.transcript",
                {"type": "suim.agent.transcript", "role": "assistant", "text": greeting, "final": True},
            )

        await publish_data(room, "suim.agent.state", {"type": "suim.agent.state", "state": "listening"})

        deadline = time.monotonic() + 30 * 60
        while time.monotonic() < deadline:
            remotes = list(room.remote_participants.values())
            if not remotes:
                log.info("room empty, leaving %s", job.room_id)
                break
            if job.linked_user_id and all(p.identity != job.linked_user_id for p in remotes):
                log.info("linked user left room=%s", job.room_id)
                break
            await asyncio.sleep(1.0)
    finally:
        await room.disconnect()
        log.info("left room=%s", job.room_id)


def apply_rtc_yaml_fallbacks() -> None:
    rtc_yaml = Path(__file__).resolve().parents[3] / "services/rtc/etc/rtc.yaml"
    if not rtc_yaml.is_file():
        return
    try:
        import yaml

        data = yaml.safe_load(rtc_yaml.read_text(encoding="utf-8")) or {}
    except Exception:
        log.exception("read rtc.yaml")
        return
    lk = data.get("livekit") or {}
    redis_cfg = data.get("redis") or {}
    agents = data.get("agents") or {}
    if lk.get("secret"):
        os.environ.setdefault("LIVEKIT_API_SECRET", str(lk["secret"]))
    if lk.get("url"):
        os.environ.setdefault("LIVEKIT_URL", str(lk["url"]))
    if lk.get("key"):
        os.environ.setdefault("LIVEKIT_API_KEY", str(lk["key"]))
    if agents.get("stream"):
        os.environ.setdefault("AGENT_STREAM", str(agents["stream"]))
    password = redis_cfg.get("password") or ""
    addr = (redis_cfg.get("address") or ["127.0.0.1:6379"])[0]
    auth = f":{password}@" if password else ""
    os.environ.setdefault("REDIS_URL", f"redis://{auth}{addr}/0")


async def consume(cfg: RuntimeConfig) -> None:
    models: ModelBundle | None = None
    if cfg.siliconflow.api_key:
        try:
            models = build_models(cfg.siliconflow)
            log.info(
                "siliconflow ready llm=%s asr=%s tts=%s",
                cfg.siliconflow.llm_model,
                cfg.siliconflow.asr_model,
                cfg.siliconflow.tts_model,
            )
        except Exception:
            log.exception("model init failed; continuing without TTS/LLM")
            models = None
    else:
        log.warning("SILICONFLOW_API_KEY empty — join-only mode (no TTS/LLM)")

    rdb = redis.from_url(cfg.redis_url, decode_responses=True)
    stream = cfg.agent_stream
    group = cfg.agent_group
    consumer = f"worker-{os.getpid()}"
    try:
        await rdb.xgroup_create(stream, group, id="0", mkstream=True)
    except redis.ResponseError as e:
        if "BUSYGROUP" not in str(e):
            raise
    log.info("listening stream=%s group=%s", stream, group)

    stop = asyncio.Event()

    def _stop(*_: Any) -> None:
        stop.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _stop)
        except NotImplementedError:
            pass

    try:
        while not stop.is_set():
            rows = await rdb.xreadgroup(group, consumer, streams={stream: ">"}, count=1, block=5000)
            if not rows:
                continue
            for _name, messages in rows:
                for msg_id, fields in messages:
                    body = fields.get("body") or ""
                    try:
                        job = AgentJob.from_body(body)
                        await handle_job(cfg, models, job)
                    except Exception:
                        log.exception("job failed id=%s", msg_id)
                    finally:
                        await rdb.xack(stream, group, msg_id)
    finally:
        if models is not None:
            await models.aclose()
        await rdb.aclose()


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")
    load_dotenv()
    apply_rtc_yaml_fallbacks()
    cfg = RuntimeConfig.from_env()
    asyncio.run(consume(cfg))


if __name__ == "__main__":
    main()
