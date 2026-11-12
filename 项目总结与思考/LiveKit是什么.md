LiveKit 是一套开源、生产可用的实时音视频（RTC）基础设施，定位是 **WebRTC 服务端框架 / SFU（Selective Forwarding Unit，选择性转发单元）**，用来快速搭建低延迟的音视频通话、直播、屏幕共享、语音房间、实时互动场景，替代自己从零封装 WebRTC、信令、TURN/STUN、媒体转发的繁琐工作。

> 
> 官网：[https://livekit.io](https://livekit.io)
> GitHub：[https://github.com/livekit/livekit](https://github.com/livekit/livekit)

## 一、核心概念：什么是 SFU

WebRTC 默认是 P2P（点对点），人数一多带宽暴涨、网络兼容性差。

- **P2P**：每个人都要向所有人发送流，N 个人就是 O (N²)
- **SFU**：媒体只发给服务端，服务端选择性转发给需要观看的客户端，复杂度降到 O (N)

LiveKit 就是一个高性能 SFU，同时内置完整信令服务，客户端只需要和 LiveKit 服务建立连接，不需要自己维护信令逻辑。

## 二、主要能力

### ✅ 音视频核心

- **房间（Room）模型**：所有互动以房间为单位，用户加入房间、发布轨道（Track）、订阅别人的轨道
- **轨道分离**：视频轨、音频轨、数据轨（DataTrack），可以单独发布 / 暂停
- **自适应码率（Simulcast / SVC）**：推流端同时推多路分辨率，服务端根据接收方网络下发合适画质
- **屏幕共享、摄像头、麦克风、虚拟轨道**
- **静音、关闭视频、踢人、权限控制**

### ✅ 数据通道（DataTrack）

不止音视频，可以发送自定义二进制 / 文本消息：

- 白板同步
- 游戏状态同步
- 远程控制指令
- 聊天消息

### ✅ 媒体录制 & 转推

- 服务端录制：录制房间流到 MP4
- 转推（RTMP）：把房间流推到抖音、B 站、直播平台
- 回放、旁路输出

### ✅ 智能降噪、回声消除

客户端 SDK 依托 WebRTC 自带；服务端支持媒体处理插件（LiveKit Egress / Ingress）

### ✅ 接入协议

- **原生 WebRTC（首选）**
- **Ingress：把 RTMP / WHIP 流拉进 LiveKit 房间**（外部直播推流进来）
- **Egress：把房间流导出为 RTMP / 文件 / HLS**（房间流对外输出）

## 三、架构概览

```
客户端（JS / Flutter / Go / iOS / Android）
        ↓（WebSocket 信令 + WebRTC UDP/TCP 媒体）
┌────────────────────────────┐
│      LiveKit Server       │
│  ├─ 信令服务（房间管理）    │
│  ├─ SFU 媒体转发           │
│  ├─ 内置 STUN/TURN         │
│  └─ Auth / API / WebHook   │
└────────────────────────────┘
        ↓
Ingress（外部流入） ←─── 房间 ───→ Egress（流导出/录制）
```

- **信令**：使用 WebSocket，JSON 协议，负责加入房间、发布轨道、订阅、权限变更
- **媒体**：UDP WebRTC；网络差时自动回退 TCP
- **认证**：使用 JWT，服务端或业务后端签发 Token，Token 里携带房间、身份、权限

## 四、SDK 支持

官方全平台 SDK：

- `livekit-client`：JavaScript / TypeScript（Web、Electron）
- `livekit-server-sdk`：Go / Node.js/ Python（后端控制，创建房间、生成 token、调用 API）
- `livekit-flutter`：跨端移动端
- `livekit-ios`、`livekit-android`：原生移动端
- Rust SDK、Unity SDK、Unreal SDK

> 
> 也就是说：前端只调用 SDK，不用手写 SDP、ICE、信令交换。

## 五、两种部署方式

### 1. LiveKit Cloud（托管版）

官方 SaaS，不用运维服务器，适合快速上线；按量计费，自带全球边缘节点。

### 2. 自托管（开源版）

使用 Docker / Kubernetes 部署，完全开源免费，适合私有化部署。

最简 Docker 启动（可直接复制）

```
docker run --rm -p 7880:7880 -p 7882:7882/udp livekit/livekit-server --dev
```

> 
> `--dev`：开发模式，免 TLS、内置密钥，禁止生产使用

生产一般搭配：

- Redis：房间持久化、集群多实例共享状态
- PostgreSQL（可选）
- Nginx / Caddy 做 TLS、域名

## 六、典型业务场景

- 多人视频会议 / 在线问诊 / 线上课堂
- 语音房、实时语音聊天
- 互动直播（连麦）
- 远程协助、桌面共享
- 元宇宙、实时多人游戏音视频
- 低延迟互动白板 + 音视频一体

## 七、优势

1. **开箱即用**：不再啃 WebRTC 底层信令、ICE、TURN
2. **性能强**：用 Go 编写，低延迟，支持大规模房间
3. **生态完整**：Ingress / Egress、录制、转推、WebHook
4. **跨平台统一**：一套逻辑跑 Web / App / 游戏引擎
5. **开源可控**：可以私有化、二次修改

## 八、局限 & 需要注意

1. **不是 CDN HLS**：面向低延迟双向实时互动，不适合超大规模纯观看直播（万人纯观看更适合 HLS）
2. **带宽成本**：SFU 需要服务器带宽，人数多成本上升
3. **生产必须 TLS**：WebRTC 在浏览器强制 HTTPS
4. **JWT 权限需要业务做好**：Token 如果泄露，别人可以冒用身份进房间

## 九、最小 Web Demo（JS）

```
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>LiveKit Demo</title>
</head>
<body>
  <script type="module">
    import { Room } from "https://cdn.jsdelivr.net/npm/@livekit/client@latest/+esm";
    const room = new Room();
    await room.connect("ws://localhost:7880", "你的 jwt token");
    console.log("加入房间成功", room.name);
  </script>
</body>
</html>
```

## 十、和同类项目简单对比

表格

| 项目 | 定位 | 特点 |
| --- | --- | --- |
| LiveKit | 完整 SFU + 信令 + SDK | 生态最好，多端 SDK、Ingress/Egress |
| Janus | SFU | 插件化，偏底层，上手更难 |
| Mediasoup | SFU 库 | 只有底层库，需要自己写全套信令 |
| Pion | Go WebRTC 库 | 基础库，需要自己搭建上层 |

> 
> 一句话总结：如果你不想自己造信令、房间、转发轮子，想要一套直接对接前端 SDK 的 RTC 服务，优先选 LiveKit。