from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path


def _env(name: str, default: str = "") -> str:
    return (os.environ.get(name) or default).strip()


def load_dotenv(path: Path | None = None) -> None:
    """Load KEY=VALUE lines into os.environ if the key is not already set."""
    candidates = []
    if path is not None:
        candidates.append(path)
    here = Path(__file__).resolve().parent.parent
    candidates.extend([here / ".env", here.parent.parent / ".env"])
    for file in candidates:
        if not file.is_file():
            continue
        for raw in file.read_text(encoding="utf-8").splitlines():
            line = raw.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, val = line.partition("=")
            key = key.strip()
            if key and key not in os.environ:
                os.environ[key] = val.strip().strip('"').strip("'")
        break


@dataclass(frozen=True)
class SiliconFlowConfig:
    api_key: str
    base_url: str = "https://api.siliconflow.cn/v1"
    llm_model: str = "Qwen/Qwen2.5-7B-Instruct"
    asr_model: str = "FunAudioLLM/SenseVoiceSmall"
    tts_model: str = "FunAudioLLM/CosyVoice2-0.5B"
    tts_voice: str = "FunAudioLLM/CosyVoice2-0.5B:bella"
    timeout_s: float = 60.0

    @classmethod
    def from_env(cls) -> SiliconFlowConfig:
        return cls(
            api_key=_env("SILICONFLOW_API_KEY"),
            base_url=_env("SILICONFLOW_BASE_URL", "https://api.siliconflow.cn/v1").rstrip("/"),
            llm_model=_env("SILICONFLOW_LLM_MODEL", "Qwen/Qwen2.5-7B-Instruct"),
            asr_model=_env("SILICONFLOW_ASR_MODEL", "FunAudioLLM/SenseVoiceSmall"),
            tts_model=_env("SILICONFLOW_TTS_MODEL", "FunAudioLLM/CosyVoice2-0.5B"),
            tts_voice=_env("SILICONFLOW_TTS_VOICE", "FunAudioLLM/CosyVoice2-0.5B:bella"),
            timeout_s=float(_env("SILICONFLOW_TIMEOUT_S", "60") or "60"),
        )

    def require_key(self) -> None:
        if not self.api_key:
            raise RuntimeError(
                "SILICONFLOW_API_KEY is empty. Create a key at https://cloud.siliconflow.cn/ "
                "and put it in agents/voice-assistant/.env"
            )


@dataclass(frozen=True)
class RuntimeConfig:
    livekit_url: str
    livekit_api_key: str
    livekit_api_secret: str
    redis_url: str
    agent_stream: str
    agent_group: str
    siliconflow: SiliconFlowConfig

    @classmethod
    def from_env(cls) -> RuntimeConfig:
        load_dotenv()
        return cls(
            livekit_url=_env("LIVEKIT_URL", "ws://127.0.0.1:7880"),
            livekit_api_key=_env("LIVEKIT_API_KEY", "suim"),
            livekit_api_secret=_env("LIVEKIT_API_SECRET"),
            redis_url=_env("REDIS_URL", "redis://127.0.0.1:6379/0"),
            agent_stream=_env("AGENT_STREAM", "suim:agent:dispatch"),
            agent_group=_env("AGENT_GROUP", "suim-agent-workers"),
            siliconflow=SiliconFlowConfig.from_env(),
        )
