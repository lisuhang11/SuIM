// ============================================================
// WebSocket 连接管理器 — 对接 SuIM Message Gateway (port 9001)
// ============================================================
// ============================================================
// SuIM SDK — WebSocket long connection
// ============================================================
import { getToken } from "@/services/storage";
import { toMessage } from "../core/rest";
import type { FriendRequestPushPayload, WsMessage, WsMessageType } from "@/types";

type MessageHandler = (msg: WsMessage) => void;
type StatusHandler = (connected: boolean) => void;

let wsBase = process.env.NEXT_PUBLIC_WS_URL || "ws://localhost:9001/ws";
const HEARTBEAT_INTERVAL = 30_000; // 30s
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000, 30000]; // 指数退避

export function setWsBase(base: string): void {
  wsBase = base.replace(/\/$/, "");
}

export function getWsBase(): string {
  return wsBase;
}

/** 网关原始帧：{ type, seq_id?, data? } */
interface GatewayFrame {
  type?: string;
  seq_id?: string;
  data?: unknown;
  payload?: unknown;
  timestamp?: string;
  err_code?: number;
  err_msg?: string;
}

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
      const url = `${wsBase}?token=${encodeURIComponent(token)}`;
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

    // 网关认 heartbeat；其余仍走前端约定帧
    if (type === "heartbeat" || type === "ping") {
      this.ws.send(JSON.stringify({ type: "heartbeat" }));
      return true;
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

  /** 向本地订阅者分发 SDK 事件（不经 WebSocket 发送） */
  emit(type: WsMessageType, payload: unknown): void {
    this.dispatch({
      type,
      payload,
      timestamp: new Date().toISOString(),
    });
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
      const frame = JSON.parse(event.data) as GatewayFrame;
      const normalized = this.normalizeFrame(frame);
      for (const msg of normalized) {
        this.dispatch(msg);
      }
    } catch {
      console.warn("[WS] Failed to parse message");
    }
  }

  /** 将网关帧规范化为前端 WsMessage */
  private normalizeFrame(frame: GatewayFrame): WsMessage[] {
    const type = String(frame.type || "");
    const now = new Date().toISOString();

    if (type === "heartbeat" || type === "pong") {
      return [];
    }

    if (type === "kick") {
      return [{
        type: "kick",
        payload: frame.data ?? frame.payload ?? null,
        timestamp: now,
      }];
    }

    // 网关在线推送：{ type: "push", data: MsgData }
    if (type === "push") {
      const data = this.asRecord(frame.data ?? frame.payload);
      if (!data) return [];
      const contentType = Number(data.content_type ?? data.contentType ?? 0);

      // 好友申请 / 同意 / 拒绝 → 刷申请列表
      if (contentType === 1000 || contentType === 1002) {
        const tips = this.parseTips(data);
        return [{
          type: "friend.request",
          payload: tips,
          timestamp: now,
        }];
      }
      // 同意好友 / 删除 / 备注置顶 / 对方资料变更 → IncrSyncFriends
      if (
        contentType === 1001 ||
        contentType === 1003 ||
        contentType === 1004 ||
        contentType === 1005
      ) {
        const tips = this.parseTips(data);
        const out: WsMessage[] = [];
        if (contentType === 1001) {
          out.push({
            type: "friend.request",
            payload: tips,
            timestamp: now,
          });
        }
        out.push({
          type: "friend.sync",
          payload: { contentType, tips },
          timestamp: now,
        });
        return out;
      }

      // 群系统事件 contentType=1100 → 消息展示 + 触发已加入群增量
      if (contentType === 1100) {
        let eventType = "";
        let groupId = "";
        try {
          const parsed = JSON.parse(String(data.content ?? "")) as Record<string, unknown>;
          eventType = String(parsed.type ?? "");
          groupId = String(parsed.group_id ?? parsed.groupId ?? "");
        } catch {
          // ignore
        }
        const out: WsMessage[] = [
          {
            type: "message.new",
            payload: { message: toMessage(data) },
            timestamp: now,
          },
        ];
        const joinSyncTypes = new Set([
          "group.created",
          "group.members_joined",
          "group.application_accepted",
          "group.member_kicked",
          "group.member_quit",
          "group.dismissed",
          "group.updated",
          "group.owner_transferred",
          "group.muted",
          "group.unmuted",
        ]);
        const memberSyncTypes = new Set([
          "group.created",
          "group.members_joined",
          "group.application_accepted",
          "group.member_kicked",
          "group.member_quit",
          "group.dismissed",
          "group.updated",
          "group.owner_transferred",
          "group.muted",
          "group.unmuted",
          "group.member_muted",
          "group.member_unmuted",
        ]);
        if (joinSyncTypes.has(eventType)) {
          out.push({
            type: "group.sync",
            payload: { eventType, groupId },
            timestamp: now,
          });
        }
        if (memberSyncTypes.has(eventType) && groupId) {
          out.push({
            type: "group.member.sync",
            payload: { eventType, groupId },
            timestamp: now,
          });
        }
        return out;
      }

      // 通话 tip contentType 1401–1407
      if (contentType >= 1401 && contentType <= 1407) {
        const tips = this.parseJSONContent(data);
        const eventMap: Record<number, string> = {
          1401: "call.invite",
          1402: "call.accepted",
          1403: "call.rejected",
          1404: "call.cancelled",
          1405: "call.timeout",
          1406: "call.busy",
          1407: "call.ended",
        };
        const eventType = eventMap[contentType];
        if (!eventType) return [];
        return [{
          type: eventType as WsMessageType,
          payload: {
            callId: String(tips.call_id ?? tips.callId ?? ""),
            callerId: String(tips.caller_id ?? tips.callerId ?? "") || undefined,
            calleeId: String(tips.callee_id ?? tips.calleeId ?? "") || undefined,
            mediaType: String(tips.media_type ?? tips.mediaType ?? "") || undefined,
            conversationId: String(tips.conversation_id ?? tips.conversationId ?? "") || undefined,
            reason: tips.reason ? String(tips.reason) : undefined,
            durationSec: Number(tips.duration_sec ?? tips.durationSec ?? 0) || undefined,
          },
          timestamp: now,
        }];
      }

      // 在线状态 tip contentType=1303
      if (contentType === 1303) {
        const tips = this.parseJSONContent(data);
        const statusNum = Number(tips.status ?? 0);
        return [{
          type: "user.status",
          payload: {
            userId: String(tips.user_id ?? tips.userId ?? ""),
            status: statusNum === 1 ? "online" : "offline",
            platformIds: tips.platform_ids ?? tips.platformIds,
          },
          timestamp: now,
        }];
      }

      // 撤回 tip contentType=2101（对齐 OpenIM MsgRevokeNotification）
      if (contentType === 2101) {
        const tips = this.parseJSONContent(data);
        return [{
          type: "message.revoke",
          payload: {
            conversationId: String(
              tips.conversation_id ?? tips.conversationId ?? data.conversation_id ?? data.conversationId ?? ""
            ),
            clientMsgId: String(tips.client_msg_id ?? tips.clientMsgId ?? ""),
            seq: Number(tips.seq ?? 0) || undefined,
            revokerUserId: String(tips.revoker_user_id ?? tips.revokerUserId ?? data.send_id ?? data.sendId ?? ""),
            revokeTime: Number(tips.revoke_time ?? tips.revokeTime ?? 0) || undefined,
            sessionType: Number(tips.session_type ?? tips.sessionType ?? data.session_type ?? 0) || undefined,
          },
          timestamp: now,
        }];
      }

      // 已读 tip contentType=2200（对齐 OpenIM HasReadReceipt）
      if (contentType === 2200) {
        const tips = this.parseJSONContent(data);
        const seqsRaw = tips.seqs;
        const seqs = Array.isArray(seqsRaw)
          ? seqsRaw.map((n) => Number(n)).filter((n) => Number.isFinite(n) && n > 0)
          : [];
        return [{
          type: "message.read",
          payload: {
            conversationId: String(
              tips.conversation_id ?? tips.conversationId ?? data.conversation_id ?? data.conversationId ?? ""
            ),
            userId: String(
              tips.mark_as_read_user_id ?? tips.markAsReadUserId ?? data.send_id ?? data.sendId ?? ""
            ),
            hasReadSeq: Number(tips.has_read_seq ?? tips.hasReadSeq ?? 0) || 0,
            seqs,
          },
          timestamp: now,
        }];
      }

      // 普通聊天消息
      return [{
        type: "message.new",
        payload: { message: toMessage(data) },
        timestamp: now,
      }];
    }

    // 订阅后的在线状态快照
    if (type === "presence.snapshot") {
      const data = this.asRecord(frame.data ?? frame.payload);
      const statuses = Array.isArray(data?.statuses) ? data!.statuses : [];
      const out: WsMessage[] = [];
      for (const raw of statuses) {
        const item = this.asRecord(raw);
        if (!item) continue;
        const statusNum = Number(item.status ?? 0);
        out.push({
          type: "user.status",
          payload: {
            userId: String(item.user_id ?? item.userId ?? ""),
            status: statusNum === 1 ? "online" : "offline",
            platformIds: item.platform_ids ?? item.platformIds,
          },
          timestamp: now,
        });
      }
      return out;
    }

    // 已是前端约定帧
    if (type) {
      return [{
        type: type as WsMessageType,
        payload: frame.payload ?? frame.data ?? null,
        timestamp: frame.timestamp || now,
      }];
    }

    return [];
  }

  private parseJSONContent(data: Record<string, unknown>): Record<string, unknown> {
    const content = String(data.content ?? "");
    if (!content) return {};
    try {
      const parsed = JSON.parse(content) as unknown;
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        return parsed as Record<string, unknown>;
      }
    } catch {
      // ignore
    }
    return {};
  }

  private parseTips(data: Record<string, unknown>): FriendRequestPushPayload {
    const parsed = this.parseJSONContent(data);
    if (Object.keys(parsed).length > 0) {
      return {
        from_user_id: String(parsed.from_user_id ?? parsed.fromUserId ?? ""),
        to_user_id: String(parsed.to_user_id ?? parsed.toUserId ?? ""),
        apply_msg: parsed.apply_msg != null ? String(parsed.apply_msg) : parsed.applyMsg != null ? String(parsed.applyMsg) : undefined,
        apply_time: Number(parsed.apply_time ?? parsed.applyTime ?? 0) || undefined,
        handle_msg: parsed.handle_msg != null ? String(parsed.handle_msg) : parsed.handleMsg != null ? String(parsed.handleMsg) : undefined,
        handle_time: Number(parsed.handle_time ?? parsed.handleTime ?? 0) || undefined,
      };
    }
    return {
      from_user_id: String(data.send_id ?? data.sendId ?? ""),
      to_user_id: String(data.recv_id ?? data.recvId ?? ""),
    };
  }

  private asRecord(value: unknown): Record<string, unknown> | null {
    if (!value) return null;
    if (typeof value === "string") {
      try {
        const parsed = JSON.parse(value) as unknown;
        return parsed && typeof parsed === "object" && !Array.isArray(parsed)
          ? (parsed as Record<string, unknown>)
          : null;
      } catch {
        return null;
      }
    }
    if (typeof value === "object" && !Array.isArray(value)) {
      return value as Record<string, unknown>;
    }
    return null;
  }

  private dispatch(msg: WsMessage): void {
    const typeHandlers = this.handlers.get(msg.type);
    if (typeHandlers) {
      typeHandlers.forEach((fn) => fn(msg));
    }
    const wildcard = this.handlers.get("*" as WsMessageType);
    if (wildcard) {
      wildcard.forEach((fn) => fn(msg));
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
    const ws = this.ws;
    const readyState = ws?.readyState;
    const states = ["CONNECTING", "OPEN", "CLOSING", "CLOSED"];
    console.warn(
      `[WS] Error (readyState=${readyState != null ? states[readyState] ?? readyState : "null"}), will retry...`,
      event
    );
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: "heartbeat" }));
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

    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
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
