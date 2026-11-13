# SuIM LiveKit Agents

**Moss-Live Path B**：在现有 Redis 派发 + `AgentSession` 上做全双工话轮（TurnDetector + 自适应打断 + 播放回执），而不是直连 Moss API。

## 链路

1. 客户端呼叫用户 ID `suim-agent`（见 `rtc.yaml` → `agents.botUserIds`）
2. `services/rtc` 跳过好友校验，自动 Accept，并向呼叫方返回 `OpAccept`（含 LiveKit URL/Token）
3. rtc `XADD` 到 `suim:agent:dispatch`
4. `agents/voice-assistant/worker.py` 消费任务，以 `agent:assistant` 进房
5. 在 `fake_job_context` 下启动 LiveKit Agents `AgentSession`
6. **Silero VAD + TurnDetector（默认本地 `v1-mini`）+ adaptive interruption**；STT/LLM 走 SiliconFlow；TTS 用 edge-tts
7. 字幕：`suim.agent.transcript`；播放进度：`suim.agent.playback`（双向）

## 模块

| 文件 | 作用 |
|------|------|
| `voice_pipeline.py` | AgentSession + turn_handling / 打断 / 续说 hint |
| `playback_tracker.py` | run 生命周期 + 客户端 `played_ms` + 打断位点 |
| `tool_router.py` | 前台 sync / async 工具脚手架（P2） |
| `delegation_worker.py` | 后台委派脚手架（P3） |
| `edge_tts_plugin.py` | SuIM TTS（edge-tts） |
| `worker.py` | Redis 消费 + 进房 |

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

前端：联系人 → AI 助手 → 语音通话。接通后应听到问候；助手说话时可插话打断；说「继续」应从断点附近接着说。

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

双工相关环境变量见 `voice-assistant/.env.example`。

## 验收（P0/P1）

- [ ] 助手播报时麦克风仍在传，插话后立刻停播
- [ ] 短停顿（犹豫）不轻易抢答（TurnDetector + dynamic endpointing）
- [ ] 「嗯/啊」等附和不易误打断（adaptive + false interruption resume）
- [ ] 前端回传 `playback.progress` / `playback.stopped`；worker 日志可见打断 `played_ms`
- [ ] 打断后说「继续」，不从头完整重念

## 下一阶段

- P2：把 `tool_router` 挂到 Agent tools（async 查单不堵对话）
- P3：`delegation_worker` 接长任务，进度可口播
- P4：视频帧 → 多模态；可选流式 TTS 压首包

## 助手音色（SuIM 自有）

- 目录：apigateway 内置中文 edge-tts 音色列表（不依赖外部音色 API）
- 偏好：Redis `suim:agent:voice:{userID}` = edge-tts voice 名（如 `zh-CN-YunxiNeural`）
- 试听：`preview_tts.py` 本机合成短句，经 `POST /agent/voices/preview` 返回
- 通话：rtc 派发 job 带 `voice_id` → worker 用该音色跑 edge-tts
