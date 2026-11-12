"""LiveKit Agents AgentSession pipeline (STT → LLM → TTS) for SuIM voice bot.

Keeps SuIM's Redis room dispatch: worker connects the room itself, then runs
AgentSession under fake_job_context (no LiveKit Cloud / AgentServer required).
"""

from __future__ import annotations

import asyncio
import logging
import os
from collections.abc import Awaitable, Callable
from typing import Any

from livekit import rtc
from livekit.agents import Agent, AgentSession, room_io
from livekit.agents.testing import fake_job_context
from livekit.agents.voice import ConversationItemAddedEvent, UserInputTranscribedEvent
from livekit.plugins import openai, silero

import edge_tts_plugin

LOG = logging.getLogger("suim-agent.voice")

GREETING = os.environ.get("AGENT_GREETING", "你好，我是 SuIM 语音助手。有什么可以帮你？")
INSTRUCTIONS = os.environ.get(
    "AGENT_INSTRUCTIONS",
    "你是 SuIM 即时通讯里的语音助手。用简洁自然的中文口语回答，一两句为主，不要用 markdown。",
)
OPENAI_API_KEY = os.environ.get("OPENAI_API_KEY", "").strip()
OPENAI_BASE_URL = os.environ.get("OPENAI_BASE_URL", "").strip() or None
OPENAI_MODEL = os.environ.get("OPENAI_MODEL", "gpt-4o-mini")
OPENAI_STT_MODEL = os.environ.get("OPENAI_STT_MODEL", "whisper-1")
# SenseVoice / some gateways reject unknown language=; detect_language omits it.
STT_DETECT_LANGUAGE = os.environ.get("OPENAI_STT_DETECT_LANGUAGE", "1").strip() not in {
    "0",
    "false",
    "False",
    "",
}
STT_LANGUAGE = os.environ.get("OPENAI_STT_LANGUAGE", "").strip()
EDGE_VOICE = os.environ.get("AGENT_TTS_VOICE", "zh-CN-XiaoxiaoNeural")
TTS_SAMPLE_RATE = int(os.environ.get("AGENT_TTS_SAMPLE_RATE", "24000"))

_VAD: Any | None = None


def has_llm() -> bool:
    return bool(OPENAI_API_KEY)


def _require_key() -> None:
    if not OPENAI_API_KEY:
        raise RuntimeError(
            "OPENAI_API_KEY is required for AgentSession (SiliconFlow or OpenAI-compatible)"
        )


def _vad() -> Any:
    global _VAD
    if _VAD is None:
        LOG.info("loading Silero VAD")
        _VAD = silero.VAD.load()
    return _VAD


def build_session() -> AgentSession:
    _require_key()
    stt_kwargs: dict[str, Any] = {
        "model": OPENAI_STT_MODEL,
        "api_key": OPENAI_API_KEY,
        "use_realtime": False,
    }
    if OPENAI_BASE_URL:
        stt_kwargs["base_url"] = OPENAI_BASE_URL
    if STT_DETECT_LANGUAGE or not STT_LANGUAGE:
        stt_kwargs["detect_language"] = True
    else:
        stt_kwargs["language"] = STT_LANGUAGE

    llm_kwargs: dict[str, Any] = {
        "model": OPENAI_MODEL,
        "api_key": OPENAI_API_KEY,
        "temperature": 0.7,
    }
    if OPENAI_BASE_URL:
        llm_kwargs["base_url"] = OPENAI_BASE_URL

    return AgentSession(
        vad=_vad(),
        stt=openai.STT(**stt_kwargs),
        llm=openai.LLM(**llm_kwargs),
        tts=edge_tts_plugin.TTS(voice=EDGE_VOICE, sample_rate=TTS_SAMPLE_RATE),
    )


def _item_text(item: Any) -> str:
    text = getattr(item, "text_content", None)
    if callable(text):
        text = text()
    if isinstance(text, str):
        return text.strip()
    content = getattr(item, "content", None)
    if isinstance(content, list):
        parts = [c for c in content if isinstance(c, str)]
        return "".join(parts).strip()
    return ""


async def run_agent_session(
    room: rtc.Room,
    *,
    linked_user_id: str = "",
    on_transcript: Callable[[str, str], Awaitable[None]] | None = None,
    greeting: str | None = None,
) -> None:
    """Run AgentSession until the linked user leaves / room closes / session ends."""
    session = build_session()
    done = asyncio.Event()

    def _wire_transcripts() -> None:
        if on_transcript is None:
            return

        @session.on("user_input_transcribed")
        def _on_user(ev: UserInputTranscribedEvent) -> None:
            if not ev.is_final:
                return
            text = (ev.transcript or "").strip()
            if not text:
                return
            asyncio.create_task(on_transcript("user", text))

        @session.on("conversation_item_added")
        def _on_item(ev: ConversationItemAddedEvent) -> None:
            item = ev.item
            role = getattr(item, "role", None)
            if role != "assistant":
                return
            text = _item_text(item)
            if not text:
                return
            asyncio.create_task(on_transcript("assistant", text))

        @session.on("close")
        def _on_close(_ev: Any = None) -> None:
            done.set()

    _wire_transcripts()

    if linked_user_id:
        room_options = room_io.RoomOptions(
            participant_identity=linked_user_id,
            close_on_disconnect=True,
            delete_room_on_close=False,
        )
    else:
        room_options = room_io.RoomOptions(close_on_disconnect=True, delete_room_on_close=False)

    identity = ""
    try:
        identity = room.local_participant.identity or "agent:assistant"
    except Exception:  # noqa: BLE001
        identity = "agent:assistant"

    with fake_job_context(room=room, agent_identity=identity):
        await session.start(
            agent=Agent(instructions=INSTRUCTIONS),
            room=room,
            room_options=room_options,
        )
        line = (greeting if greeting is not None else GREETING).strip()
        if line:
            await session.say(line, allow_interruptions=True)
        LOG.info("AgentSession running; waiting for close")
        try:
            await done.wait()
        finally:
            await session.aclose()
