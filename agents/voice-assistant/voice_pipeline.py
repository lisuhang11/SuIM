"""LiveKit Agents AgentSession pipeline (Moss-Live Path B) for SuIM voice bot.

Keeps SuIM's Redis room dispatch: worker connects the room itself, then runs
AgentSession under fake_job_context (no LiveKit Cloud / AgentServer required).

Path B P0/P1:
- Silero VAD + local TurnDetector (v1-mini) for semantic endpointing
- Adaptive interruption + false-interruption resume + backchannel boundary
- PlaybackTracker for run lifecycle / client played_ms / interrupt resume hint
"""

from __future__ import annotations

import asyncio
import logging
import os
from collections.abc import Awaitable, Callable
from typing import Any

from livekit import rtc
from livekit.agents import Agent, AgentSession, inference, room_io
from livekit.agents.testing import fake_job_context
from livekit.agents.utils import http_context
from livekit.agents.voice import (
    AgentFalseInterruptionEvent,
    ConversationItemAddedEvent,
    SpeechCreatedEvent,
    UserInputTranscribedEvent,
)
from livekit.plugins import openai, silero

import edge_tts_plugin
from playback_tracker import PlaybackTracker

LOG = logging.getLogger("suim-agent.voice")

GREETING = os.environ.get("AGENT_GREETING", "你好，我是 SuIM 语音助手。有什么可以帮你？")
INSTRUCTIONS = os.environ.get(
    "AGENT_INSTRUCTIONS",
    (
        "你是 SuIM 即时通讯里的语音助手，语气像打电话：简洁自然的中文口语，一两句为主，不要用 markdown。"
        "用户插话或换话题时立刻跟新内容；若用户说继续/接着说，从上一轮被打断处往下讲，不要从头完整重念。"
        "短暂的嗯、啊、喂可当作附和，不要当成新问题长篇回答。"
    ),
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

# Turn / interruption knobs (Moss-Live Path B)
TURN_DETECTOR = os.environ.get("AGENT_TURN_DETECTOR", "1").strip() not in {"0", "false", "False"}
TURN_DETECTOR_VERSION = os.environ.get("AGENT_TURN_DETECTOR_VERSION", "v1-mini").strip() or "v1-mini"
INTERRUPTION_MODE = os.environ.get("AGENT_INTERRUPTION_MODE", "adaptive").strip() or "adaptive"
ENDPOINTING_MODE = os.environ.get("AGENT_ENDPOINTING_MODE", "dynamic").strip() or "dynamic"
MIN_ENDPOINTING_DELAY = float(os.environ.get("AGENT_MIN_ENDPOINTING_DELAY", "0.35"))
MAX_ENDPOINTING_DELAY = float(os.environ.get("AGENT_MAX_ENDPOINTING_DELAY", "2.5"))
MIN_INTERRUPTION_DURATION = float(os.environ.get("AGENT_MIN_INTERRUPTION_DURATION", "0.35"))
FALSE_INTERRUPTION_TIMEOUT = float(os.environ.get("AGENT_FALSE_INTERRUPTION_TIMEOUT", "1.5"))

_VAD: Any | None = None
_TURN: Any | None = None


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


def _turn_detector() -> Any | None:
    global _TURN
    if not TURN_DETECTOR:
        return None
    if _TURN is None:
        LOG.info("loading TurnDetector version=%s", TURN_DETECTOR_VERSION)
        # v1-mini runs locally; v1 needs LiveKit Inference credentials.
        _TURN = inference.TurnDetector(version=TURN_DETECTOR_VERSION)  # type: ignore[arg-type]
    return _TURN


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

    turn_detection = _turn_detector()
    interruption: dict[str, Any] = {
        "enabled": True,
        "mode": INTERRUPTION_MODE if INTERRUPTION_MODE in {"adaptive", "vad"} else "adaptive",
        "min_duration": MIN_INTERRUPTION_DURATION,
        "min_words": 0,
        "resume_false_interruption": True,
        "false_interruption_timeout": FALSE_INTERRUPTION_TIMEOUT,
        "backchannel_boundary": (1.0, 1.0),
        "discard_audio_if_uninterruptible": True,
    }
    endpointing: dict[str, Any] = {
        "mode": ENDPOINTING_MODE if ENDPOINTING_MODE in {"fixed", "dynamic"} else "dynamic",
        "min_delay": MIN_ENDPOINTING_DELAY,
        "max_delay": MAX_ENDPOINTING_DELAY,
    }
    turn_handling: dict[str, Any] = {
        "endpointing": endpointing,
        "interruption": interruption,
    }
    if turn_detection is not None:
        turn_handling["turn_detection"] = turn_detection

    LOG.info(
        "AgentSession duplex turn_handling detector=%s interruption=%s endpointing=%s",
        TURN_DETECTOR_VERSION if turn_detection else "off",
        interruption["mode"],
        endpointing["mode"],
    )
    return AgentSession(
        vad=_vad(),
        stt=openai.STT(**stt_kwargs),
        llm=openai.LLM(**llm_kwargs),
        tts=edge_tts_plugin.TTS(voice=EDGE_VOICE, sample_rate=TTS_SAMPLE_RATE),
        turn_handling=turn_handling,
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


def _speech_text(handle: Any) -> str:
    items = getattr(handle, "chat_items", None) or []
    parts: list[str] = []
    for item in items:
        role = getattr(item, "role", None)
        if role and role != "assistant":
            continue
        t = _item_text(item)
        if t:
            parts.append(t)
    return "".join(parts).strip()


async def run_agent_session(
    room: rtc.Room,
    *,
    linked_user_id: str = "",
    on_transcript: Callable[[str, str], Awaitable[None]] | None = None,
    on_publish: Callable[[str, str], Awaitable[None]] | None = None,
    playback: PlaybackTracker | None = None,
    greeting: str | None = None,
) -> None:
    """Run AgentSession until the linked user leaves / room closes / session ends."""
    session = build_session()
    done = asyncio.Event()
    tracker = playback or PlaybackTracker(publish=on_publish)
    if on_publish is not None:
        tracker.bind_publish(on_publish)

    base_instructions = INSTRUCTIONS
    agent_ref: dict[str, Any] = {}

    def _wire_events() -> None:
        @session.on("speech_created")
        def _on_speech(ev: SpeechCreatedEvent) -> None:
            handle = ev.speech_handle
            run_id = getattr(handle, "id", "") or f"speech-{id(handle)}"

            async def _started() -> None:
                # chat_items may fill as synthesis progresses; seed empty then refresh on done.
                await tracker.on_speech_started(run_id, text=_speech_text(handle))

            asyncio.create_task(_started())

            def _on_done(_h: Any = None) -> None:
                interrupted = bool(getattr(handle, "interrupted", False))

                async def _finish() -> None:
                    text = _speech_text(handle)
                    tracker.set_text(run_id, text)
                    await tracker.on_speech_finished(run_id, interrupted=interrupted)
                    if interrupted:
                        hint = tracker.resume_hint()
                        if hint:
                            try:
                                await agent_ref["agent"].update_instructions(
                                    f"{base_instructions}\n{hint}".strip()
                                )
                            except Exception:  # noqa: BLE001
                                LOG.debug("resume hint update failed", exc_info=True)

                asyncio.create_task(_finish())

            handle.add_done_callback(_on_done)

        @session.on("agent_false_interruption")
        def _on_false(_ev: AgentFalseInterruptionEvent) -> None:
            LOG.info("false interruption (resume=%s)", getattr(_ev, "resumed", None))

        if on_transcript is not None:

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

    _wire_events()

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

    # fake_job_context alone does not bind aiohttp; adaptive interruption / TurnDetector
    # need http_context.open() when not running under AgentServer.
    async with http_context.open():
        with fake_job_context(room=room, agent_identity=identity):
            agent = Agent(instructions=base_instructions)
            agent_ref["agent"] = agent
            await session.start(
                agent=agent,
                room=room,
                room_options=room_options,
            )
            line = (greeting if greeting is not None else GREETING).strip()
            if line:
                await session.say(line, allow_interruptions=True)
            LOG.info("AgentSession duplex running; waiting for close")

            async def _hint_loop() -> None:
                last = ""
                while not done.is_set():
                    await asyncio.sleep(0.8)
                    h = tracker.resume_hint()
                    if h and h != last:
                        last = h
                        try:
                            await agent.update_instructions(f"{base_instructions}\n{h}".strip())
                        except Exception:  # noqa: BLE001
                            LOG.debug("update_instructions failed", exc_info=True)

            hint_task = asyncio.create_task(_hint_loop(), name="resume-hint")
            try:
                await done.wait()
            finally:
                hint_task.cancel()
                try:
                    await hint_task
                except asyncio.CancelledError:
                    pass
                await session.aclose()
