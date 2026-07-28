// ============================================================
// WebSocket 连接管理器 — 对接 SuIM Message Gateway (port 9001)
// ============================================================
import { getToken } from "./storage";
import type { WsMessage, WsMessageType } from "@/types";

type MessageHandler = (msg: WsMessage) => void;
type StatusHandler = (connected: boolean) => void;

const WS_BASE = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:9001/ws";
const HEARTBEAT_INTERVAL = 30_000; // 30s
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000]; // 指数退避

class WebSocketManager {
  private ws: WebSocket | null = null;
  private handlers: Map<WsMessageType, Set<MessageHandler>> = new Map();
  private statusHandlers: Set<StatusHandler> = new Set();
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempt = 0;
  private shouldReconnect = true;
  private isConnected = false;

  /** 建立 WebSocket 连接 */
  connect(): void {
    const token = getToken();
    if (!token) {
      console.warn("[WS] No token, skip connect");
      return;
    }

    // 断开已有连接（切换账号时）
    if (this.ws) {
      this.shouldReconnect = false;
      if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close(1000, "Reconnect");
      }
      this.ws = null;
      this.setConnected(false);
    }

    this.shouldReconnect = true;

    try {
      const url = `${WS_BASE}?token=${encodeURIComponent(token)}`;
      console.log("[WS] Connecting...");
      this.ws = new WebSocket(url);
      this.ws.onopen = this.onOpen.bind(this);
      this.ws.onmessage = this.onMessage.bind(this);
      this.ws.onclose = this.onClose.bind(this);
      this.ws.onerror = this.onError.bind(this);
    } catch (err) {
      console.error("[WS] Constructor error:", err);
      this.scheduleReconnect();
    }
  }

  /** 断开连接 */
  disconnect(): void {
    this.shouldReconnect = false;
    this.clearTimers();

    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onclose = null;
      this.ws.onerror = null;
      if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close(1000, "Client disconnect");
      }
      this.ws = null;
    }

    this.setConnected(false);
    this.reconnectAttempt = 0;
  }

  /** 发送消息到服务端 */
  send(type: WsMessageType, payload: unknown): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn("[WS] Not connected, cannot send:", type);
      return false;
    }

    const msg: WsMessage = {
      type,
      payload,
      timestamp: new Date().toISOString(),
    };

    this.ws.send(JSON.stringify(msg));
    return true;
  }

  /** 注册消息处理器 */
  on(type: WsMessageType, handler: MessageHandler): () => void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set());
    }
    this.handlers.get(type)!.add(handler);

    return () => {
      this.handlers.get(type)?.delete(handler);
    };
  }

  /** 注册连接状态变化回调 */
  onStatusChange(handler: StatusHandler): () => void {
    this.statusHandlers.add(handler);
    // 立即通知当前状态
    handler(this.isConnected);
    return () => {
      this.statusHandlers.delete(handler);
    };
  }

  /** 获取当前连接状态 */
  get connected(): boolean {
    return this.isConnected;
  }

  // ---------- 内部方法 ----------
  private onOpen(): void {
    console.log("[WS] Connected");
    this.setConnected(true);
    this.reconnectAttempt = 0;
    this.startHeartbeat();
  }

  private onMessage(event: MessageEvent): void {
    try {
      const msg = JSON.parse(event.data) as WsMessage;
      if (msg.type === "pong") return; // 心跳响应，不广播

      // 分发到注册的处理器
      const typeHandlers = this.handlers.get(msg.type);
      if (typeHandlers) {
        typeHandlers.forEach((fn) => fn(msg));
      }

      // 也分发给通配符 "*" 处理器
      const wildcard = this.handlers.get("*" as WsMessageType);
      if (wildcard) {
        wildcard.forEach((fn) => fn(msg));
      }
    } catch {
      console.warn("[WS] Failed to parse message");
    }
  }

  private onClose(event: CloseEvent): void {
    console.log(`[WS] Closed: code=${event.code}, reason=${event.reason}`);
    this.setConnected(false);
    this.stopHeartbeat();

    if (this.shouldReconnect && event.code !== 1000) {
      this.scheduleReconnect();
    }
  }

  private onError(event: Event): void {
    // 连接失败是预期行为（网络波动、token 过期等），自动重连会处理
    // 降级为 warn 避免污染控制台
    const ws = this.ws;
    const readyState = ws?.readyState;
    const states = ["CONNECTING", "OPEN", "CLOSING", "CLOSED"];
    console.warn(
      `[WS] Error (readyState=${readyState != null ? states[readyState] ?? readyState : "null"}), will retry...`,
      event
    );
    // onClose will be called after onError → scheduleReconnect
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: "ping", payload: null, timestamp: new Date().toISOString() }));
      }
    }, HEARTBEAT_INTERVAL);
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  private scheduleReconnect(): void {
    const delay = RECONNECT_DELAYS[Math.min(this.reconnectAttempt, RECONNECT_DELAYS.length - 1)];
    console.log(`[WS] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempt + 1})`);

    this.reconnectTimer = setTimeout(() => {
      this.reconnectAttempt++;
      this.connect();
    }, delay);
  }

  private clearTimers(): void {
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  private setConnected(connected: boolean): void {
    if (this.isConnected === connected) return;
    this.isConnected = connected;
    this.statusHandlers.forEach((fn) => fn(connected));
  }
}

// 单例
export const wsManager = new WebSocketManager();
