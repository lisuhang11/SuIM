# SuIM Agents

自建语音助手 worker（不调用 Moss API）。媒体走 LiveKit，模型走可插拔 provider（默认 [SiliconFlow](https://cloud.siliconflow.cn/)）。

```
rtc ──Redis Stream──► agents/voice-assistant
                         │
                         ├─ models/siliconflow  (LLM / ASR / TTS)
                         └─ LiveKit 进房对讲
```

详见 [`voice-assistant/README.md`](voice-assistant/README.md)。
