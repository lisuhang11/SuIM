from __future__ import annotations

from dataclasses import dataclass

from voice_assistant.config import SiliconFlowConfig
from voice_assistant.models.base import ASR, LLM, TTS
from voice_assistant.models.siliconflow import SiliconFlowASR, SiliconFlowLLM, SiliconFlowTTS


@dataclass
class ModelBundle:
    llm: LLM
    asr: ASR
    tts: TTS
    provider: str = "siliconflow"

    async def aclose(self) -> None:
        for part in (self.llm, self.asr, self.tts):
            close = getattr(part, "aclose", None)
            if close is not None:
                await close()


def build_models(cfg: SiliconFlowConfig | None = None, *, provider: str = "siliconflow") -> ModelBundle:
    """Build ASR/LLM/TTS. Only siliconflow is implemented for now."""
    provider = (provider or "siliconflow").strip().lower()
    if provider != "siliconflow":
        raise ValueError(f"unsupported model provider: {provider}")
    cfg = cfg or SiliconFlowConfig.from_env()
    cfg.require_key()
    return ModelBundle(
        llm=SiliconFlowLLM(cfg),
        asr=SiliconFlowASR(cfg),
        tts=SiliconFlowTTS(cfg),
        provider="siliconflow",
    )
