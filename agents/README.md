# SuIM LiveKit Agents

Phase 1：rtc 在「呼叫 AI bot」接通后向 Redis Stream 投递任务，本目录 worker 进 LiveKit 房间。

## 链路

1. 客户端呼叫用户 ID `suim-agent`（见 `rtc.yaml` → `agents.botUserIds`）
2. `services/rtc` 跳过好友校验，自动 Accept，并向呼叫方推送 `OpAccept`（含 LiveKit URL/Token）
3. rtc `XADD` 到 `suim:agent:dispatch`
4. `agents/voice-assistant/worker.py` 消费任务，以 `agent:assistant` 身份进房

## 启动

先保证 LiveKit、Redis、rtc 已运行，然后：

```bash
chmod +x agents/voice-assistant/run.sh
./agents/voice-assistant/run.sh
```

前端：我 → 「呼叫 AI 助手」，或任意处 `startVoice('suim-agent')`。

## 配置

`services/rtc/etc/rtc.yaml`：

```yaml
agents:
  enabled: true
  stream: suim:agent:dispatch
  identity: agent:assistant
  botUserIds:
    - suim-agent
  joinOnAccept: false   # true 时普通人接听也会拉 Agent（调试用）
```

## 下一阶段

- 接入 LiveKit Agents（STT → LLM → TTS）真正对讲
- 群通话助手、转写字幕
- tool 回写 IM 消息
