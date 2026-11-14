from __future__ import annotations

import json

import httpx
import pytest

from voice_assistant.config import SiliconFlowConfig
from voice_assistant.models.base import ChatMessage
from voice_assistant.models.siliconflow.asr import SiliconFlowASR
from voice_assistant.models.siliconflow.tts import SiliconFlowTTS


def _cfg() -> SiliconFlowConfig:
    return SiliconFlowConfig(
        api_key="test-key",
        base_url="https://api.siliconflow.cn/v1",
        llm_model="Qwen/Qwen2.5-7B-Instruct",
        asr_model="FunAudioLLM/SenseVoiceSmall",
        tts_model="FunAudioLLM/CosyVoice2-0.5B",
        tts_voice="FunAudioLLM/CosyVoice2-0.5B:bella",
    )


@pytest.mark.asyncio
async def test_asr_posts_multipart() -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        captured["auth"] = request.headers.get("Authorization")
        captured["content_type"] = request.headers.get("Content-Type", "")
        return httpx.Response(200, json={"text": "你好世界"})

    transport = httpx.MockTransport(handler)
    headers = {"Authorization": "Bearer test-key"}
    async with httpx.AsyncClient(
        transport=transport,
        base_url="https://api.siliconflow.cn/v1/",
        headers=headers,
    ) as client:
        asr = SiliconFlowASR(_cfg(), client=client)
        text = await asr.transcribe(b"RIFF....", filename="clip.wav", mime="audio/wav")
    assert text == "你好世界"
    assert captured["url"].endswith("/audio/transcriptions")
    assert captured["auth"] == "Bearer test-key"
    assert "multipart/form-data" in captured["content_type"]


@pytest.mark.asyncio
async def test_tts_posts_json_and_returns_bytes() -> None:
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        captured["url"] = str(request.url)
        captured["body"] = json.loads(request.content.decode())
        return httpx.Response(200, content=b"\x00\x01PCM", headers={"content-type": "audio/pcm"})

    transport = httpx.MockTransport(handler)
    async with httpx.AsyncClient(transport=transport, base_url="https://api.siliconflow.cn/v1/") as client:
        tts = SiliconFlowTTS(_cfg(), client=client)
        out = await tts.synthesize("你好", response_format="pcm", sample_rate=24000)
    assert out.audio == b"\x00\x01PCM"
    assert out.mime == "audio/pcm"
    assert captured["body"]["model"] == "FunAudioLLM/CosyVoice2-0.5B"
    assert captured["body"]["voice"] == "FunAudioLLM/CosyVoice2-0.5B:bella"
    assert captured["body"]["response_format"] == "pcm"
    assert captured["url"].endswith("/audio/speech")


@pytest.mark.asyncio
async def test_tts_empty_text_skips_http() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise AssertionError("should not call API for empty text")

    transport = httpx.MockTransport(handler)
    async with httpx.AsyncClient(transport=transport, base_url="https://api.siliconflow.cn/v1/") as client:
        tts = SiliconFlowTTS(_cfg(), client=client)
        out = await tts.synthesize("  ")
    assert out.audio == b""


def test_chat_message_roundtrip_shape() -> None:
    msg = ChatMessage(role="user", content="hi")
    assert msg.role == "user"
