# SuIM LiveKit Agents

Phase 2：rtc 派发任务后，worker 进 LiveKit 房间，用语音打招呼，并在配置了模型后进行对讲。

## 链路

1. 客户端呼叫用户 ID `suim-agent`（见 `rtc.yaml` → `agents.botUserIds`）
2. `services/rtc` 跳过好友校验，自动 Accept，并向呼叫方推送 `OpAccept`（含 LiveKit URL/Token）
3. rtc `XADD` 到 `suim:agent:dispatch`
4. `agents/voice-assistant/worker.py` 消费任务，以 `agent:assistant` 进房
5. **edge-tts** 播报问候（无需付费 key）
6. 能量 VAD 检测你说完后：若配置了 `OPENAI_API_KEY`，走 Whisper STT → Chat LLM → TTS 回复

## 启动

先保证 LiveKit、Redis、rtc 已运行，然后：

```bash
cd agents/voice-assistant
# 可选：写入模型密钥（不要提交）
cat > .env <<'ENV'
OPENAI_API_KEY=sk-...
# SiliconFlow 示例：
OPENAI_BASE_URL=https://api.siliconflow.cn/v1
OPENAI_MODEL=Qwen/Qwen2.5-7B-Instruct
OPENAI_STT_MODEL=FunAudioLLM/SenseVoiceSmall
ENV
./run.sh
```

前端：我 → 「呼叫 AI 助手」。接通后应能听到中文问候；配置 key 后可继续对话。

没有 `OPENAI_API_KEY` 时：仍能听到问候；你说话后助手会用语音提示如何配置模型。

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

## 下一阶段

- 接入 LiveKit Agents 官方插件 / Inference（更稳的端点检测与打断）
- 群通话助手、转写字幕（前端已可订阅 data topic `suim.agent.transcript`）
- tool 回写 IM 消息
