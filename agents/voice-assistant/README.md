# SuIM Voice Assistant

自建全双工语音助手 worker。模型默认走 [SiliconFlow](https://cloud.siliconflow.cn/)（OpenAI 兼容）。

## 目录

```
voice_assistant/
  config.py                 # 环境变量
  models/
    base.py                 # LLM / ASR / TTS 协议
    factory.py              # build_models()
    siliconflow/            # SiliconFlow 实现
      llm.py                # chat + stream
      asr.py                # /audio/transcriptions
      tts.py                # /audio/speech
    smoke.py                # 有 Key 时的连通性探测
  worker.py                 # Redis job → LiveKit 进房 + 问候
```

## 配置

```bash
cp .env.example .env
# 填入 SILICONFLOW_API_KEY（控制台 → API Keys）
```

默认模型（均可改）：

| 能力 | 默认 |
|------|------|
| LLM | `Qwen/Qwen2.5-7B-Instruct` |
| ASR | `FunAudioLLM/SenseVoiceSmall` |
| TTS | `FunAudioLLM/CosyVoice2-0.5B` + `…:bella` |
| Base URL | `https://api.siliconflow.cn/v1` |

LiveKit / Redis 若未写在 `.env`，worker 会从 `services/rtc/etc/rtc.yaml` 补全。

## 命令

```bash
# 安装依赖并启动 worker（优先用 uv 建 venv）
./run.sh

# 仅装依赖 / 跑测试（不连网）
uv venv .venv && uv pip install -r requirements.txt
.venv/bin/pytest -q

# SiliconFlow 连通性（需要 Key）
.venv/bin/python -m voice_assistant.models.smoke
```

## 当前进度

- [x] SiliconFlow model 层（LLM / ASR / TTS）
- [x] worker 进房 + LLM/TTS 问候
- [ ] 用户音轨流式 ASR → LLM → TTS 对讲
- [ ] barge-in + playback 回执
