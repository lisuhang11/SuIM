from __future__ import annotations

import httpx

from voice_assistant.config import SiliconFlowConfig
from voice_assistant.models.base import TTSResult
from voice_assistant.models.siliconflow.client import http_client, raise_for_api

_MIME = {
    "mp3": "audio/mpeg",
    "opus": "audio/opus",
    "wav": "audio/wav",
    "pcm": "audio/pcm",
}


class SiliconFlowTTS:
    """POST /audio/speech (CosyVoice2 / fish-speech)."""

    def __init__(self, cfg: SiliconFlowConfig, client: httpx.AsyncClient | None = None) -> None:
        self._cfg = cfg
        self._client = client
        self._owns_client = client is None
        self.model = cfg.tts_model
        self.default_voice = cfg.tts_voice

    async def _http(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = http_client(self._cfg)
        return self._client

    async def aclose(self) -> None:
        if self._owns_client and self._client is not None:
            await self._client.aclose()
            self._client = None

    async def synthesize(
        self,
        text: str,
        *,
        voice: str | None = None,
        response_format: str = "pcm",
        sample_rate: int = 24000,
        speed: float = 1.0,
    ) -> TTSResult:
        text = (text or "").strip()
        if not text:
            return TTSResult(audio=b"", mime=_MIME.get(response_format, "application/octet-stream"), sample_rate=sample_rate)

        payload = {
            "model": self.model,
            "input": text,
            "voice": (voice or self.default_voice).strip(),
            "response_format": response_format,
            "sample_rate": sample_rate,
            "stream": False,
            "speed": speed,
        }
        client = await self._http()
        resp = await client.post("audio/speech", json=payload)
        raise_for_api(resp)
        mime = resp.headers.get("content-type") or _MIME.get(response_format, "application/octet-stream")
        return TTSResult(audio=resp.content, mime=mime.split(";")[0].strip(), sample_rate=sample_rate)
