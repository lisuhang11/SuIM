from __future__ import annotations

from collections.abc import AsyncIterator, Sequence
from typing import Any

from openai import AsyncOpenAI

from voice_assistant.config import SiliconFlowConfig
from voice_assistant.models.base import ChatMessage


class SiliconFlowLLM:
    """OpenAI-compatible chat completions against api.siliconflow.cn."""

    def __init__(self, cfg: SiliconFlowConfig, client: AsyncOpenAI | None = None) -> None:
        cfg.require_key()
        self._cfg = cfg
        self._owns_client = client is None
        self._client = client or AsyncOpenAI(
            api_key=cfg.api_key,
            base_url=cfg.base_url,
            timeout=cfg.timeout_s,
        )
        self.model = cfg.llm_model

    async def aclose(self) -> None:
        if self._owns_client:
            await self._client.close()

    def _payload(self, messages: Sequence[ChatMessage]) -> list[dict[str, str]]:
        out: list[dict[str, str]] = []
        for m in messages:
            role = m.role.strip() or "user"
            if role not in ("system", "user", "assistant"):
                role = "user"
            out.append({"role": role, "content": m.content})
        return out

    async def chat(self, messages: Sequence[ChatMessage], *, temperature: float = 0.7) -> str:
        resp = await self._client.chat.completions.create(
            model=self.model,
            messages=self._payload(messages),  # type: ignore[arg-type]
            temperature=temperature,
            stream=False,
        )
        choice = resp.choices[0].message
        return (choice.content or "").strip()

    async def stream(self, messages: Sequence[ChatMessage], *, temperature: float = 0.7) -> AsyncIterator[str]:
        stream = await self._client.chat.completions.create(
            model=self.model,
            messages=self._payload(messages),  # type: ignore[arg-type]
            temperature=temperature,
            stream=True,
        )
        async for chunk in stream:
            if not chunk.choices:
                continue
            delta: Any = chunk.choices[0].delta
            text = getattr(delta, "content", None) or ""
            if text:
                yield text
