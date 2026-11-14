from __future__ import annotations

from dataclasses import dataclass
from typing import AsyncIterator, Protocol, Sequence


@dataclass(frozen=True)
class ChatMessage:
    role: str  # system | user | assistant
    content: str


@dataclass(frozen=True)
class TTSResult:
    audio: bytes
    mime: str
    sample_rate: int | None = None


class LLM(Protocol):
    async def chat(self, messages: Sequence[ChatMessage], *, temperature: float = 0.7) -> str:
        """Non-streaming completion."""

    def stream(self, messages: Sequence[ChatMessage], *, temperature: float = 0.7) -> AsyncIterator[str]:
        """Yield text deltas."""


class ASR(Protocol):
    async def transcribe(
        self,
        audio: bytes,
        *,
        filename: str = "audio.wav",
        mime: str = "audio/wav",
        language: str | None = None,
    ) -> str:
        """Transcribe one audio blob to text."""


class TTS(Protocol):
    async def synthesize(
        self,
        text: str,
        *,
        voice: str | None = None,
        response_format: str = "pcm",
        sample_rate: int = 24000,
        speed: float = 1.0,
    ) -> TTSResult:
        """Synthesize speech; prefer pcm for LiveKit publishing."""
