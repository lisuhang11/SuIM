#!/usr/bin/env python3
"""Optional live smoke against SiliconFlow (needs SILICONFLOW_API_KEY).

Usage:
  cd agents/voice-assistant
  .venv/bin/python -m voice_assistant.models.smoke
"""

from __future__ import annotations

import asyncio
import sys
import wave
from io import BytesIO

from voice_assistant.config import SiliconFlowConfig, load_dotenv
from voice_assistant.models.base import ChatMessage
from voice_assistant.models.factory import build_models


def _silent_wav(seconds: float = 0.4, rate: int = 16000) -> bytes:
    frames = int(rate * seconds)
    buf = BytesIO()
    with wave.open(buf, "wb") as wf:
        wf.setnchannels(1)
        wf.setsampwidth(2)
        wf.setframerate(rate)
        wf.writeframes(b"\x00\x00" * frames)
    return buf.getvalue()


async def main() -> int:
    load_dotenv()
    cfg = SiliconFlowConfig.from_env()
    if not cfg.api_key:
        print("SKIP: SILICONFLOW_API_KEY not set", file=sys.stderr)
        return 0
    models = build_models(cfg)
    try:
        print(f"provider=siliconflow base={cfg.base_url}")
        print(f"llm={cfg.llm_model} asr={cfg.asr_model} tts={cfg.tts_model}")

        reply = await models.llm.chat(
            [
                ChatMessage(role="system", content="你是简洁的中文助手，用一句话回答。"),
                ChatMessage(role="user", content="用四个字打个招呼。"),
            ],
            temperature=0.3,
        )
        print(f"llm: {reply!r}")

        parts: list[str] = []
        async for delta in models.llm.stream(
            [ChatMessage(role="user", content="数到三，只用数字和逗号。")],
            temperature=0.2,
        ):
            parts.append(delta)
        print(f"llm_stream: {''.join(parts)!r}")

        text = await models.asr.transcribe(_silent_wav(), filename="silence.wav")
        print(f"asr(silence): {text!r}")

        spoken = reply or "你好，我是苏IM语音助手。"
        audio = await models.tts.synthesize(spoken[:40], response_format="pcm", sample_rate=24000)
        print(f"tts: bytes={len(audio.audio)} mime={audio.mime} rate={audio.sample_rate}")
        print("OK")
        return 0
    finally:
        await models.aclose()


if __name__ == "__main__":
    raise SystemExit(asyncio.run(main()))
