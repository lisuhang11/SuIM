# SuIM LiveKit Agents

Phase 2（官方 `AgentSession`）：rtc 派发任务后，worker 进 LiveKit 房间，用 **Silero VAD + OpenAI 兼容 STT/LLM + edge-tts** 对讲（可打断）。

## 链路

1. 客户端呼叫用户 ID `suim-agent`（见 `rtc.yaml` → `agents.botUserIds`）
2. `services/rtc` 跳过好友校验，自动 Accept，并向呼叫方返回 `OpAccept`（含 LiveKit URL/Token）
3. rtc `XADD` 到 `suim:agent:dispatch`
4. `agents/voice-assistant/worker.py` 消费任务，以 `agent:assistant` 进房
5. 在 `fake_job_context` 下启动 LiveKit Agents `AgentSession`（保留 Redis 派发，不必上 LiveKit Cloud AgentServer）
6. Silero 端点检测 + 官方打断；STT/LLM 走 SiliconFlow（或其它 OpenAI 兼容网关）；TTS 用 edge-tts（免 TTS 密钥）
7. 字幕经 data topic `suim.agent.transcript` 推到前端

## 启动

先保证 LiveKit、Redis、rtc 已运行，然后：

```bash
cd agents/voice-assistant
# 必填：模型密钥（不要提交）
cat > .env <<'ENV'
OPENAI_API_KEY=sk-...
OPENAI_BASE_URL=https://api.siliconflow.cn/v1
OPENAI_MODEL=Qwen/Qwen2.5-7B-Instruct
OPENAI_STT_MODEL=FunAudioLLM/SenseVoiceSmall
OPENAI_STT_DETECT_LANGUAGE=1
ENV
./run.sh
```

前端：联系人 → AI 助手 → 语音通话。接通后应听到问候，并可连续对话、打断助手播报。

## 配置

`services/rtc/etc/rtc.yaml`：

```yaml
agents:
  enabled: true
  stream: suim:agent:dispatch
  identity: agent:assistant
  botUserIds:
    - suim-agent
  joinOnAccept: false
```

## 与旧版差异

| | 旧自研能量 VAD | 现 AgentSession |
|--|----------------|-----------------|
| 端点检测 | RMS 能量 | Silero VAD |
| 打断 | 助手说话时丢弃用户音频 | 框架原生 barge-in |
| 编排 | 手写 STT→LLM→TTS 循环 | `AgentSession` |
| 派发 | Redis（不变） | Redis + `fake_job_context` |

## 下一阶段

- 可选：改为 LiveKit 原生 Agent Dispatch（`AgentServer` / `agent_name`），去掉 Redis
- 会话记忆落库、tool 回写 IM
- 流式 TTS 插件（进一步压首包延迟）
