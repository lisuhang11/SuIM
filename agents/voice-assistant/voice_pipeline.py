"""Local STT → LLM → TTS pipeline over a LiveKit room (Phase 2).

Uses edge-tts for speech out. STT/LLM use OpenAI-compatible APIs (e.g. SiliconFlow).
"""

from __future__ import annotations

import asyncio
import io
import logging
import os
import struct
import wave
from typing import Any, Awaitable, Callable

import av
import edge_tts
import numpy as np
from livekit import rtc

LOG = logging.getLogger("suim-agent.voice")

SAMPLE_RATE = 48000
NUM_CHANNELS = 1
FRAME_MS = 20
SAMPLES_PER_FRAME = SAMPLE_RATE * FRAME_MS // 1000

EDGE_VOICE = os.environ.get("AGENT_TTS_VOICE", "zh-CN-XiaoxiaoNeural")
GREETING = os.environ.get("AGENT_GREETING", "你好，我是 SuIM 语音助手。有什么可以帮你？")
INSTRUCTIONS = os.environ.get(
    "AGENT_INSTRUCTIONS",
    "你是 SuIM 即时通讯里的语音助手。用简洁自然的中文口语回答，一两句为主，不要用 markdown。",
)
OPENAI_API_KEY = os.environ.get("OPENAI_API_KEY", "").strip()
OPENAI_BASE_URL = os.environ.get("OPENAI_BASE_URL", "").strip() or None
OPENAI_MODEL = os.environ.get("OPENAI_MODEL", "gpt-4o-mini")
OPENAI_STT_MODEL = os.environ.get("OPENAI_STT_MODEL", "whisper-1")
# SiliconFlow SenseVoice rejects unknown fields like language=zh.
STT_SEND_LANGUAGE = os.environ.get("OPENAI_STT_LANGUAGE", "").strip()

SPEECH_RMS = float(os.environ.get("AGENT_VAD_RMS", "0.008"))
SILENCE_MS = int(os.environ.get("AGENT_SILENCE_MS", "800"))
MIN_SPEECH_MS = int(os.environ.get("AGENT_MIN_SPEECH_MS", "300"))
MAX_UTTERANCE_MS = int(os.environ.get("AGENT_MAX_UTTERANCE_MS", "15000"))


def has_llm() -> bool:
    return bool(OPENAI_API_KEY)


def _openai_client():
    from openai import OpenAI

    kwargs: dict[str, Any] = {"api_key": OPENAI_API_KEY}
    if OPENAI_BASE_URL:
        kwargs["base_url"] = OPENAI_BASE_URL
    return OpenAI(**kwargs)


async def synthesize_pcm(text: str, sample_rate: int = SAMPLE_RATE) -> bytes:
    text = (text or "").strip()
    if not text:
        return b""
    communicate = edge_tts.Communicate(text, EDGE_VOICE)
    mp3 = bytearray()
    async for chunk in communicate.stream():
        if chunk["type"] == "audio":
            mp3.extend(chunk["data"])
    if not mp3:
        return b""
    container = av.open(io.BytesIO(bytes(mp3)), format="mp3")
    resampler = av.audio.resampler.AudioResampler(format="s16", layout="mono", rate=sample_rate)
    pcm = bytearray()
    for frame in container.decode(audio=0):
        for out in resampler.resample(frame):
            pcm.extend(bytes(out.planes[0]))
    for out in resampler.resample(None):
        pcm.extend(bytes(out.planes[0]))
    return bytes(pcm)


def pcm_to_wav(pcm: bytes, sample_rate: int = SAMPLE_RATE) -> bytes:
    buf = io.BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(sample_rate)
        wf.writeframes(pcm)
    return buf.getvalue()


async def transcribe(pcm: bytes) -> str:
    if not has_llm() or not pcm:
        return ""

    def _run() -> str:
        client = _openai_client()
        wav = pcm_to_wav(pcm)
        bio = io.BytesIO(wav)
        bio.name = "utterance.wav"
        kwargs: dict[str, Any] = {"model": OPENAI_STT_MODEL, "file": bio}
        if STT_SEND_LANGUAGE:
            kwargs["language"] = STT_SEND_LANGUAGE
        resp = client.audio.transcriptions.create(**kwargs)
        return (getattr(resp, "text", None) or "").strip()

    return await asyncio.to_thread(_run)


async def chat_reply(history: list[dict[str, str]], user_text: str) -> str:
    if not user_text.strip():
        return ""
    if not has_llm():
        return "我已经听到你了。要开启完整对话，请给助手配置 OPENAI_API_KEY。"

    def _run() -> str:
        client = _openai_client()
        messages = [{"role": "system", "content": INSTRUCTIONS}]
        messages.extend(history[-12:])
        messages.append({"role": "user", "content": user_text})
        resp = client.chat.completions.create(
            model=OPENAI_MODEL,
            messages=messages,
            temperature=0.7,
            max_tokens=200,
        )
        return (resp.choices[0].message.content or "").strip()

    return await asyncio.to_thread(_run)


class RoomVoice:
    """Publish TTS and run a simple energy-VAD conversation loop."""

    def __init__(
        self,
        room: rtc.Room,
        *,
        linked_user_id: str,
        on_transcript: Callable[[str, str], Awaitable[None]] | None = None,
    ) -> None:
        self.room = room
        self.linked = linked_user_id
        self.on_transcript = on_transcript
        self.source = rtc.AudioSource(SAMPLE_RATE, NUM_CHANNELS)
        self.track = rtc.LocalAudioTrack.create_audio_track("agent-mic", self.source)
        self._history: list[dict[str, str]] = []
        self._speaking = asyncio.Lock()
        self._stop = asyncio.Event()
        self._greeted = False

    async def start(self) -> None:
        opts = rtc.TrackPublishOptions()
        opts.source = rtc.TrackSource.SOURCE_MICROPHONE
        pub = await self.room.local_participant.publish_track(self.track, opts)
        LOG.info("published agent audio track sid=%s", getattr(pub, "sid", ""))
        # Wait for the caller so the greeting is not played into an empty room.
        participant = await self._wait_participant(timeout_s=45)
        if participant is None:
            LOG.warning("caller never joined; greeting anyway")
        else:
            LOG.info("caller present identity=%s; waiting for mic", participant.identity)
            await self._wait_audio_track(participant, timeout_s=20)
            await asyncio.sleep(0.4)
        await self.say(GREETING, role="assistant")
        self._greeted = True

    def stop(self) -> None:
        self._stop.set()

    async def say(self, text: str, *, role: str = "assistant") -> None:
        text = (text or "").strip()
        if not text:
            return
        async with self._speaking:
            LOG.info("say role=%s text=%s", role, text[:80])
            if self.on_transcript:
                try:
                    await self.on_transcript(role, text)
                except Exception:  # noqa: BLE001
                    LOG.exception("transcript callback failed")
            pcm = await synthesize_pcm(text)
            LOG.info("tts pcm_bytes=%d", len(pcm))
            await self._play_pcm(pcm)

    async def _play_pcm(self, pcm: bytes) -> None:
        if not pcm:
            return
        frame_bytes = SAMPLES_PER_FRAME * 2
        if len(pcm) % frame_bytes:
            pcm += b"\x00" * (frame_bytes - len(pcm) % frame_bytes)
        mv = memoryview(pcm)
        for i in range(0, len(mv), frame_bytes):
            if self._stop.is_set():
                break
            chunk = bytes(mv[i : i + frame_bytes])
            frame = rtc.AudioFrame(chunk, SAMPLE_RATE, NUM_CHANNELS, SAMPLES_PER_FRAME)
            await self.source.capture_frame(frame)
        # Let the last frames drain.
        await asyncio.sleep(0.05)

    async def run_loop(self) -> None:
        participant = await self._wait_participant(timeout_s=60)
        if participant is None:
            LOG.warning("listen aborted: no caller")
            return
        track = await self._wait_audio_track(participant, timeout_s=60)
        if track is None:
            LOG.warning("listen aborted: no mic track")
            return
        LOG.info("listening to remote audio from %s", participant.identity)
        stream = rtc.AudioStream.from_track(
            track=track,
            sample_rate=SAMPLE_RATE,
            num_channels=NUM_CHANNELS,
        )
        silence_needed = max(1, SILENCE_MS // FRAME_MS)
        min_speech = max(1, MIN_SPEECH_MS // FRAME_MS)
        max_utt = max(min_speech, MAX_UTTERANCE_MS // FRAME_MS)

        buf = bytearray()
        speech_frames = 0
        silence_frames = 0
        in_speech = False

        try:
            async for event in stream:
                if self._stop.is_set():
                    break
                if self._speaking.locked():
                    in_speech = False
                    speech_frames = 0
                    silence_frames = 0
                    buf.clear()
                    continue
                frame = event.frame
                data = bytes(frame.data)
                rms = _rms(data)
                if rms >= SPEECH_RMS:
                    in_speech = True
                    silence_frames = 0
                    speech_frames += 1
                    buf.extend(data)
                elif in_speech:
                    silence_frames += 1
                    buf.extend(data)
                    if silence_frames >= silence_needed and speech_frames >= min_speech:
                        utter = bytes(buf)
                        buf.clear()
                        in_speech = False
                        speech_frames = 0
                        silence_frames = 0
                        await self._handle_utterance(utter)
                if speech_frames >= max_utt:
                    utter = bytes(buf)
                    buf.clear()
                    in_speech = False
                    speech_frames = 0
                    silence_frames = 0
                    await self._handle_utterance(utter)
        finally:
            await stream.aclose()

    async def _handle_utterance(self, pcm: bytes) -> None:
        if len(pcm) < SAMPLE_RATE * 2 // 10:
            return
        LOG.info("utterance pcm_bytes=%d", len(pcm))
        try:
            text = await transcribe(pcm)
        except Exception:  # noqa: BLE001
            LOG.exception("stt failed")
            await self.say("我没听清，请再说一次。")
            return
        if not text:
            LOG.info("stt empty")
            if not has_llm():
                await self.say("我在这儿。请配置模型密钥后再试对话。")
            return
        LOG.info("user said: %s", text)
        if self.on_transcript:
            await self.on_transcript("user", text)
        try:
            reply = await chat_reply(self._history, text)
        except Exception:  # noqa: BLE001
            LOG.exception("llm failed")
            await self.say("脑子短路了一下，请稍后再试。")
            return
        self._history.append({"role": "user", "content": text})
        if reply:
            self._history.append({"role": "assistant", "content": reply})
            await self.say(reply)

    async def _wait_participant(self, timeout_s: float = 30) -> rtc.RemoteParticipant | None:
        deadline = asyncio.get_event_loop().time() + timeout_s
        while asyncio.get_event_loop().time() < deadline:
            if self._stop.is_set():
                return None
            for p in self.room.remote_participants.values():
                if not self.linked or p.identity == self.linked:
                    return p
            await asyncio.sleep(0.1)
        return None

    async def _wait_audio_track(
        self, participant: rtc.RemoteParticipant, timeout_s: float = 30
    ) -> rtc.Track | None:
        deadline = asyncio.get_event_loop().time() + timeout_s
        while asyncio.get_event_loop().time() < deadline:
            if self._stop.is_set():
                return None
            for pub in participant.track_publications.values():
                if pub.kind == rtc.TrackKind.KIND_AUDIO and pub.track is not None:
                    if hasattr(pub, "subscribed") and not pub.subscribed:
                        try:
                            pub.set_subscribed(True)
                        except Exception:  # noqa: BLE001
                            pass
                    if pub.track is not None:
                        return pub.track
            await asyncio.sleep(0.1)
        return None


def _rms(pcm_s16le: bytes) -> float:
    if len(pcm_s16le) < 2:
        return 0.0
    n = len(pcm_s16le) // 2
    samples = struct.unpack("<" + "h" * n, pcm_s16le[: n * 2])
    if not samples:
        return 0.0
    acc = 0.0
    for s in samples:
        v = s / 32768.0
        acc += v * v
    return float(np.sqrt(acc / len(samples)))
