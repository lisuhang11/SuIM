from __future__ import annotations

import httpx

from voice_assistant.config import SiliconFlowConfig
from voice_assistant.models.siliconflow.client import http_client, raise_for_api


class SiliconFlowASR:
    """POST /audio/transcriptions (SenseVoice / TeleSpeech)."""

    def __init__(self, cfg: SiliconFlowConfig, client: httpx.AsyncClient | None = None) -> None:
        self._cfg = cfg
        self._client = client
        self._owns_client = client is None
        self.model = cfg.asr_model

    async def _http(self) -> httpx.AsyncClient:
        if self._client is None:
            self._client = http_client(self._cfg)
        return self._client

    async def aclose(self) -> None:
        if self._owns_client and self._client is not None:
            await self._client.aclose()
            self._client = None

    async def transcribe(
        self,
        audio: bytes,
        *,
        filename: str = "audio.wav",
        mime: str = "audio/wav",
        language: str | None = None,
    ) -> str:
        if not audio:
            return ""
        client = await self._http()
        data: dict[str, str] = {"model": self.model}
        if language:
            data["language"] = language
        files = {"file": (filename, audio, mime)}
        resp = await client.post("audio/transcriptions", data=data, files=files)
        raise_for_api(resp)
        body = resp.json()
        if isinstance(body, dict):
            return str(body.get("text") or "").strip()
        return str(body).strip()
