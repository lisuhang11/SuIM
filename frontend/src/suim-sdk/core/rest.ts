// ============================================================
// SuIM SDK — REST transport (gateway /api/v1)
// ============================================================
import { getToken } from "@/services/storage";
import { emitAuthExpired } from "@/services/auth-events";
import { parseGroupId } from "./ids";
import type {
  ApiResponse,
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  User,
  Conversation,
  Message,
  Contact,
  BlacklistEntry,
  FriendRequest,
  CreateGroupRequest,
  Group,
  GroupMemberInfo,
  GroupApplication,
  UpdateGroupRequest,
  FileAttachment,
} from "@/types";

let apiBase = process.env.NEXT_PUBLIC_API_URL || "/api/v1";
const REQUEST_TIMEOUT = 10_000; // 10 秒超时

/** Override API base (InitSDK). */
export function setApiBase(base: string): void {
  apiBase = base.replace(/\/$/, "");
}

export function getApiBase(): string {
  return apiBase;
}

function errorMessage(endpoint: string, status: number, backendMessage: string): string {
  const message = backendMessage.trim().toLowerCase();
  if (endpoint === "/users/login") {
    if (status === 401 || status === 404 || message.includes("invalid password") || message.includes("user not found")) {
      return "邮箱或密码错误";
    }
    if (message.includes("disabled") || message.includes("inactive")) return "账号已被停用";
  }
  if (endpoint === "/users/password" && (status === 401 || message.includes("invalid password"))) {
    return "当前密码错误";
  }
  if (status === 401) return "登录状态已失效，请重新登录";
  if (status === 403) return "没有权限执行此操作";
  if (status === 404) return "请求的资源不存在";
  if (status === 429) return "操作过于频繁，请稍后再试";
  if (status === 502 || status === 503 || status === 504) return "服务暂时不可用，请稍后再试";
  if (status >= 500) return "服务器处理失败，请稍后再试";

  const translations: Record<string, string> = {
    "invalid email format": "邮箱格式不正确",
    "user already exists": "该邮箱已注册",
    "new password must be different from old password": "新密码不能与当前密码相同",
    "password must be 8-32 characters with at least one letter and one number": "密码需为 8-32 位，并同时包含字母和数字",
    "already friends": "你们已经是好友了",
    "friend request already sent": "已发送过好友申请，请等待对方处理",
    "you are blocked by this user": "对方已将你拉黑，无法发送申请",
    "blocked by peer": "对方已将你拉黑，无法发送消息",
    "cannot friend yourself": "不能添加自己为好友",
    "user is already blocked": "该用户已在黑名单中",
    "user is not blocked": "该用户不在黑名单中",
  };
  return translations[message] || backendMessage || `请求失败（${status}）`;
}

// ---------- 通用请求（带超时） ----------
export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {},
  timeoutMs = REQUEST_TIMEOUT
): Promise<T> {
  return request<T>(endpoint, options, timeoutMs);
}

async function request<T>(
  endpoint: string,
  options: RequestInit = {},
  timeoutMs = REQUEST_TIMEOUT
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  const fullUrl = `${apiBase}${endpoint}`;
  if (process.env.NODE_ENV === "development") {
    console.log(`[API] ${options.method || "GET"} ${fullUrl}`);
  }

  try {
    const res = await fetch(fullUrl, {
      ...options,
      headers,
      signal: controller.signal,
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({ message: res.statusText })) as { message?: string };
      const message = errorMessage(endpoint, res.status, body.message || res.statusText);
      if (process.env.NODE_ENV === "development") {
        console.warn(`[API] ${options.method || "GET"} ${fullUrl} → ${res.status}: ${message}`);
      }
      if (res.status === 401) {
        if (typeof window !== "undefined") {
          localStorage.removeItem("suim_token");
          localStorage.removeItem("suim_user");
        }
        emitAuthExpired("unauthorized");
      }
      throw new Error(message);
    }

    if (process.env.NODE_ENV === "development") {
      console.log(`[API] ${options.method || "GET"} ${fullUrl} → ${res.status} OK`);
    }
    return res.json();
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") {
      throw new Error("请求超时，请检查后端服务");
    }
    throw err;
  } finally {
    clearTimeout(timeoutId);
  }
}

// ---------- 数据类型转换 ----------
// 后端 UserInfo -> 前端 User
function toUser(info: Record<string, unknown>): User {
  return {
    userId: String(info.user_id ?? info.userId ?? ""),
    username: String(info.nickname ?? info.username ?? ""),
    displayName: String(info.nickname ?? info.displayName ?? ""),
    avatar: String(info.avatar_url ?? info.avatarUrl ?? info.avatar ?? ""),
    email: String(info.email ?? ""),
    status: (info.status as User["status"]) || "offline",
    lastSeen: info.last_seen ? String(info.last_seen) : undefined,
    createdAt: info.create_time
      ? new Date(Number(info.create_time)).toISOString()
      : String(info.updated_at ?? info.createdAt ?? new Date().toISOString()),
  };
}

// 后端会话响应 -> 前端 Conversation
// 注意: 后端 conversation_type: 1 = 私聊(single), 2 = 群聊(group)
// 私聊后端用 user_id 表示对端，没有 members 数组。
function toConversation(raw: Record<string, unknown>): Conversation {
  const rawType = raw.conversation_type !== undefined ? Number(raw.conversation_type) : undefined;
  const type = (rawType !== undefined
    ? (rawType === 2 ? "group" : "private")
    : String(raw.type || "private")) as Conversation["type"];
  const conversationId = String(raw.conversation_id ?? raw.conversationId ?? "");
  const rawGroupId = String(raw.group_id ?? raw.groupId ?? "");
  const groupId =
    type === "group"
      ? parseGroupId(rawGroupId || conversationId)
      : undefined;
  let members: Conversation["members"] = Array.isArray(raw.members)
    ? (raw.members as Conversation["members"])
    : [];
  if (members.length === 0) {
    const peerId = String(raw.user_id ?? raw.userId ?? "");
    if (peerId && type === "private") {
      members = [{
        userId: peerId,
        conversationId,
        role: "member",
        joinedAt: String(raw.create_time ?? raw.createdAt ?? ""),
      }];
    }
  }
  return {
    conversationId,
    type,
    groupId: groupId || undefined,
    title: String(raw.title || raw.group_name || ""),
    avatar: String(raw.face_url ?? raw.avatar ?? ""),
    unreadCount: Number(raw.unread_count ?? raw.unreadCount ?? 0),
    members,
    isPinned: Boolean(raw.is_pinned ?? raw.isPinned ?? false),
    isMuted: Boolean(
      raw.is_muted ?? raw.isMuted ?? Number(raw.recv_msg_opt ?? raw.recvMsgOpt ?? 0) === 1
    ),
    lastMessage: (() => {
      const last = raw.last_message ?? raw.lastMessage ?? raw.msg_info ?? raw.msgInfo;
      if (last && typeof last === "object") {
        const m = last as Record<string, unknown>;
        // ConversationElem.MsgInfo / Conversation.msg_info → Message
        return toMessage({
          ...m,
          message_id: m.server_msg_id ?? m.serverMsgId ?? m.message_id,
          client_msg_id: m.client_msg_id ?? m.clientMsgId,
          sender_id: m.send_id ?? m.sendId ?? m.sender_id,
          sender_nickname: m.sender_name ?? m.senderName ?? m.sender_nickname,
          sender_face_url: m.face_url ?? m.faceUrl ?? m.sender_face_url,
          content_type: m.content_type ?? m.contentType,
          content: m.content,
          send_time: m.latest_msg_recv_time ?? m.latestMsgRecvTime ?? m.send_time,
          created_at: m.latest_msg_recv_time ?? m.latestMsgRecvTime ?? m.send_time,
          conversation_id: raw.conversation_id ?? raw.conversationId,
        });
      }
      return undefined;
    })(),
    createdAt: String(raw.create_time ?? raw.createdAt ?? ""),
    updatedAt: String(raw.updated_at ?? raw.updatedAt ?? ""),
  };
}

// 后端消息响应 -> 前端 Message
export function toMessage(raw: Record<string, unknown>): Message {
  // 兼容 HTTP snake_case 与 WS protojson camelCase
  const contentType = Number(raw.content_type ?? raw.contentType ?? 0);
  let content = String(raw.content ?? "");
  let file: FileAttachment | undefined;
  if (contentType === 1 || contentType === 2) {
    try {
      const parsed = JSON.parse(content) as Record<string, unknown>;
      file = {
        fileId: String(parsed.file_id ?? parsed.fileId ?? ""),
        name: String(parsed.name ?? ""),
        contentType: String(parsed.content_type ?? parsed.contentType ?? "application/octet-stream"),
        size: Number(parsed.size ?? 0),
        sha256: parsed.sha256 ? String(parsed.sha256) : undefined,
        category: String(parsed.category ?? "other") as FileAttachment["category"],
        expiresAt: String(parsed.expires_at ?? parsed.expiresAt ?? ""),
      };
      content = file.name;
    } catch {
      // Keep legacy content intact.
    }
  }
  const senderName = String(raw.sender_nickname ?? raw.senderNickname ?? raw.senderName ?? "");
  const senderId = String(raw.send_id ?? raw.sendId ?? raw.senderId ?? "");
  // 1000+：通知/系统类（含群事件 contentType=1100）
  const isSystem = contentType >= 1000;
  if (isSystem && contentType === 1501) {
    content = formatCallRecord(content);
  } else if (isSystem && contentType >= 1100) {
    content = formatGroupEventTip(content, senderName, senderId);
  }
  const sendTimeRaw = Number(raw.send_time ?? raw.sendTime ?? 0);
  // 后端一般为 Unix 毫秒；若误传秒级则补齐，避免时间落在 1970
  const sendTimeMs =
    sendTimeRaw > 0 && sendTimeRaw < 1e12 ? sendTimeRaw * 1000 : sendTimeRaw;
  const seq = Number(raw.seq ?? 0);
  const clientMsgId = String(raw.client_msg_id ?? raw.clientMsgId ?? "");
  const rawStatus = Number(raw.status ?? 0);
  const revoked = rawStatus === 1 || contentType === 2101;
  return {
    messageId: String(raw.server_msg_id ?? raw.serverMsgId ?? raw.messageId ?? clientMsgId ?? ""),
    conversationId: String(raw.conversation_id ?? raw.conversationId ?? ""),
    senderId,
    senderName,
    senderAvatar: String(raw.sender_face_url ?? raw.senderFaceUrl ?? raw.senderAvatar ?? ""),
    content: revoked ? "撤回了一条消息" : content,
    type: (revoked
      ? "system"
      : isSystem
        ? "system"
        : contentType === 1
          ? "image"
          : contentType === 2
            ? "file"
            : "text") as Message["type"],
    status: (revoked
      ? "revoked"
      : raw.is_read || raw.isRead
        ? "read"
        : "delivered") as Message["status"],
    createdAt: sendTimeMs > 0
      ? new Date(sendTimeMs).toISOString()
      : String(raw.createdAt ?? new Date().toISOString()),
    file: revoked ? undefined : file,
    seq: seq > 0 ? seq : undefined,
    clientMsgId: clientMsgId || undefined,
  };
}

/** 通话记录 contentType=1501 → 时间线文案 */
function formatCallRecord(raw: string): string {
  try {
    const parsed = JSON.parse(raw) as {
      reason?: string;
      duration_sec?: number;
      durationSec?: number;
    };
    const reason = String(parsed.reason ?? "");
    const durationSec = Number(parsed.duration_sec ?? parsed.durationSec ?? 0);
    switch (reason) {
      case "completed": {
        const m = Math.floor(durationSec / 60);
        const s = durationSec % 60;
        return `语音通话 ${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
      }
      case "rejected":
        return "已拒绝";
      case "cancelled":
        return "已取消";
      case "timeout":
        return "未接来电";
      case "busy":
        return "忙线未接通";
      case "unavailable":
        return "对方不在线";
      default:
        return reason || "语音通话";
    }
  } catch {
    return raw || "语音通话";
  }
}

/** 群事件 contentType=1100 的 JSON → 可读系统提示（对齐 OpenIM 群通知展示） */
function formatGroupEventTip(raw: string, senderName: string, senderId: string): string {
  try {
    const parsed = JSON.parse(raw) as {
      type?: string;
      operator_user_id?: string;
      subject_user_ids?: string[];
    };
    const who =
      senderName.trim() ||
      parsed.operator_user_id?.trim() ||
      senderId.trim() ||
      "未知用户";
    switch (parsed.type) {
      case "group.created":
        return `${who} 创建了群聊`;
      case "group.members_joined":
      case "group.application_accepted":
        return `${who} 邀请成员加入了群聊`;
      case "group.member_kicked":
        return `${who} 将成员移出了群聊`;
      case "group.member_quit":
        return `${who} 退出了群聊`;
      case "group.dismissed":
        return `${who} 解散了群聊`;
      default:
        return `${who} 更新了群聊`;
    }
  } catch {
    return raw;
  }
}

// ---------- 认证 ----------
// 网关路径: POST /api/v1/users/login
// backend LoginReq: { email, password }
export async function login(data: LoginRequest): Promise<AuthResponse> {
  const backendReq = {
    email: data.username, // 前端用 username 作为邮箱字段
    password: data.password,
  };
  const res = await request<ApiResponse<Record<string, unknown>>>("/users/login", {
    method: "POST",
    body: JSON.stringify(backendReq),
  });
  // backend LoginResp: { user, access_token, refresh_token }
  const d = res.data || ({} as Record<string, unknown>);
  return {
    token: String(d.access_token ?? d.token ?? ""),
    expiresAt: d.expires_at ? String(d.expires_at) : "",
    user: d.user ? toUser(d.user as Record<string, unknown>) : ({} as User),
  };
}

// 网关路径: POST /api/v1/users/register
// backend RegisterReq: { username, email, password }
export async function register(data: RegisterRequest): Promise<AuthResponse> {
  const backendReq = {
    username: data.username,
    email: data.email,
    password: data.password,
  };
  const res = await request<ApiResponse<Record<string, unknown>>>("/users/register", {
    method: "POST",
    body: JSON.stringify(backendReq),
  });
  // backend RegisterResp 没有 token，只有 user 信息
  const d = res.data || ({} as Record<string, unknown>);
  return {
    token: "", // register 通常不返回 token
    expiresAt: "",
    user: d.user ? toUser(d.user as Record<string, unknown>) : ({} as User),
  };
}

/** 从本地缓存解析当前 userId（登录后写入 suim_user）。 */
function selfUserId(): string {
  if (typeof window === "undefined") return "";
  try {
    const raw = localStorage.getItem("suim_user");
    if (!raw) return "";
    const u = JSON.parse(raw) as { userId?: string; user_id?: string };
    return String(u.userId ?? u.user_id ?? "");
  } catch {
    return "";
  }
}

// 网关路径: GET /api/v1/users/batch?ids=<self>（唯一资料读接口）
export async function getCurrentUser(): Promise<User> {
  const id = selfUserId();
  if (!id) throw new Error("未登录或缺少本地用户信息");
  const list = await getUsersBatch([id]);
  const user = list.find((u) => u.userId === id) ?? list[0];
  if (!user) throw new Error("用户不存在");
  return user;
}

// 网关路径: POST /api/v1/users/logout
export async function logout(): Promise<void> {
  await request("/users/logout", { method: "POST" });
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await request("/users/password", {
    method: "PUT",
    body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
  });
}

// ---------- 会话 ----------
/** BFF 活跃会话列表（对齐 OpenIM /jssdk/get_active_conversations） */
export type ActiveConversationsResult = {
  conversations: Conversation[];
  unreadTotal: number;
};

// 网关路径: POST /api/v1/bff/active-conversations
export async function getActiveConversations(count = 100): Promise<ActiveConversationsResult> {
  const res = await request<
    ApiResponse<{ conversations?: unknown[]; unread_total?: number; unreadTotal?: number }>
  >("/bff/active-conversations", {
    method: "POST",
    body: JSON.stringify({ count }),
  });
  const data = res.data;
  if (!data) return { conversations: [], unreadTotal: 0 };
  const list = Array.isArray(data.conversations) ? data.conversations : [];
  return {
    conversations: list.map((item) => toConversation(item as Record<string, unknown>)),
    unreadTotal: Number(data.unread_total ?? data.unreadTotal ?? 0),
  };
}

// 网关路径: POST /api/v1/bff/active-conversations（主路径）
// 兼容：旧 GET /conversations/owner 仍可用
export async function getConversations(): Promise<Conversation[]> {
  const { conversations } = await getActiveConversations();
  return conversations;
}

// 网关路径: GET /api/v1/conversations/:id?owner_user_id=...
export async function getConversation(id: string): Promise<Conversation> {
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/conversations/${id}`
  );
  return toConversation(res.data || {});
}

// 网关路径: POST /api/v1/conversations/single
// body: { recv_id }；send_id 由网关从 JWT 注入
export async function createPrivateConversation(
  userId: string
): Promise<Conversation> {
  await request("/conversations/single", {
    method: "POST",
    body: JSON.stringify({ recv_id: userId, conversation_type: 1 }),
  });
  // Create resp 为空，刷新后按对端查找
  const list = await getConversations();
  const found = list.find(
    (c) => c.type === "private" && c.members.some((m) => m.userId === userId)
  );
  if (found) return found;
  // 约定 id: si_<min>_<max>
  const a = userId;
  // 无法从客户端可靠得知自己 id 时仍返回占位，调用方应 refresh
  return {
    conversationId: "",
    type: "private",
    title: "",
    avatar: "",
    unreadCount: 0,
    members: [{ userId, conversationId: "", role: "member", joinedAt: "" }],
    isPinned: false,
    isMuted: false,
    createdAt: "",
    updatedAt: "",
  };
}

// 网关路径: POST /api/v1/conversations/group
// 仅为已有群批量建会话（内部联动用），不负责建群。需要 group_id + user_ids。
export async function createGroupConversation(
  groupId: string,
  userIds: string[]
): Promise<void> {
  await request("/conversations/group", {
    method: "POST",
    body: JSON.stringify({
      group_id: groupId,
      user_ids: userIds,
    }),
  });
}

// 网关路径: DELETE /api/v1/conversations/batch
export async function deleteConversation(id: string): Promise<void> {
  await request("/conversations/batch", {
    method: "DELETE",
    body: JSON.stringify({ conversation_ids: [id] }),
  });
}

// 网关路径: POST /api/v1/messages/read
export async function markAsRead(conversationId: string, seq: number): Promise<void> {
  await request("/messages/read", {
    method: "POST",
    body: JSON.stringify({ conversation_id: conversationId, seq }),
  });
}

// 网关路径: POST /api/v1/messages/revoke
export async function revokeMessage(conversationId: string, clientMsgId: string): Promise<void> {
  await request("/messages/revoke", {
    method: "POST",
    body: JSON.stringify({ conversation_id: conversationId, client_msg_id: clientMsgId }),
  });
}

// 网关路径: POST /api/v1/messages
export async function sendMessage(payload: {
  clientMsgId: string;
  conversationId: string;
  sessionType: number;
  groupId?: string;
  recvId?: string;
  recvUserIds?: string[];
  contentType: number;
  content: string;
  senderNickname?: string;
  senderFaceUrl?: string;
}): Promise<{ serverMsgId: string; clientMsgId: string; seq: number; sendTime: number }> {
  const res = await request<ApiResponse<{
    server_msg_id?: string;
    client_msg_id?: string;
    seq?: number;
    send_time?: number;
  }>>("/messages", {
    method: "POST",
    body: JSON.stringify({
      msg_data: {
        client_msg_id: payload.clientMsgId,
        conversation_id: payload.conversationId,
        session_type: payload.sessionType,
        group_id: payload.groupId || "",
        recv_id: payload.recvId || "",
        recv_user_ids: payload.recvUserIds || [],
        content_type: payload.contentType,
        content: payload.content,
        sender_nickname: payload.senderNickname || "",
        sender_face_url: payload.senderFaceUrl || "",
      },
    }),
  });
  return {
    serverMsgId: String(res.data?.server_msg_id || payload.clientMsgId),
    clientMsgId: String(res.data?.client_msg_id || payload.clientMsgId),
    seq: Number(res.data?.seq || 0),
    sendTime: Number(res.data?.send_time || 0),
  };
}

// 网关路径: POST /api/v1/conversations — 更新置顶 / 免打扰
export async function updateConversationSettings(
  conversation: Conversation,
  patch: { isPinned?: boolean; isMuted?: boolean },
  ownerUserId: string
): Promise<void> {
  const isPinned = patch.isPinned ?? conversation.isPinned;
  const isMuted = patch.isMuted ?? conversation.isMuted;
  await request("/conversations", {
    method: "POST",
    body: JSON.stringify({
      conversation: {
        owner_user_id: ownerUserId,
        conversation_id: conversation.conversationId,
        conversation_type: conversation.type === "group" ? 2 : 1,
        user_id: conversation.type === "private"
          ? conversation.members.find((m) => m.userId !== ownerUserId)?.userId || ""
          : "",
        group_id: conversation.type === "group"
          ? parseGroupId(conversation.groupId || conversation.conversationId)
          : "",
        is_pinned: isPinned,
        recv_msg_opt: isMuted ? 1 : 0,
      },
    }),
  });
}

// ---------- 消息 ----------
// 网关路径: GET /api/v1/messages/history?conversation_id=&seq=&limit=&order=
export async function getMessages(
  conversationId: string,
  params?: { before?: string; limit?: number }
): Promise<Message[]> {
  const query = new URLSearchParams();
  query.set("conversation_id", conversationId);
  if (params?.before) query.set("seq", params.before);
  if (params?.limit) query.set("limit", String(params.limit));
  else query.set("limit", "50");
  // order=0: newest→oldest；前端需要时间正序展示
  query.set("order", "0");
  const res = await request<ApiResponse<{
    msg_data?: Record<string, unknown>[];
    messages?: Record<string, unknown>[];
  } | Record<string, unknown>[]>>(
    `/messages/history?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data.msg_data || data.messages || []);
  const messages = list.map((item) => toMessage(item as Record<string, unknown>));
  // 后端默认 newest→oldest，翻成 oldest→newest
  return messages.reverse();
}

// 网关路径: GET /api/v1/messages/max-seqs?conversation_ids=
export async function getMaxSeqs(conversationIds?: string[]): Promise<Record<string, number>> {
  const bounds = await getMaxAndMinSeqs(conversationIds);
  const out: Record<string, number> = {};
  for (const [k, v] of Object.entries(bounds)) out[k] = v.maxSeq;
  return out;
}

/** 对齐 OpenIM GetMaxSeqResp：同时返回 max_seqs / min_seqs */
export async function getMaxAndMinSeqs(
  conversationIds?: string[]
): Promise<Record<string, { maxSeq: number; minSeq: number }>> {
  const q =
    conversationIds && conversationIds.length
      ? `?conversation_ids=${encodeURIComponent(conversationIds.join(","))}`
      : "";
  const res = await request<ApiResponse<Record<string, unknown>>>(`/messages/max-seqs${q}`);
  const raw = res.data || {};
  const maxMap = (raw.max_seqs ?? raw.maxSeqs ?? {}) as Record<string, unknown>;
  const minMap = (raw.min_seqs ?? raw.minSeqs ?? {}) as Record<string, unknown>;
  const out: Record<string, { maxSeq: number; minSeq: number }> = {};
  const keys = new Set([...Object.keys(maxMap), ...Object.keys(minMap)]);
  for (const k of keys) {
    out[k] = {
      maxSeq: Number(maxMap[k]) || 0,
      minSeq: Number(minMap[k]) || 0,
    };
  }
  return out;
}

// 网关路径: GET /api/v1/messages/has-read-and-max-seqs?conversation_ids=
export async function getHasReadAndMaxSeqs(
  conversationIds?: string[]
): Promise<Record<string, { maxSeq: number; hasReadSeq: number; maxSeqTime: number }>> {
  const q =
    conversationIds && conversationIds.length
      ? `?conversation_ids=${encodeURIComponent(conversationIds.join(","))}`
      : "";
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/messages/has-read-and-max-seqs${q}`
  );
  const raw = res.data || {};
  const map = (raw.seqs ?? {}) as Record<string, Record<string, unknown>>;
  const out: Record<string, { maxSeq: number; hasReadSeq: number; maxSeqTime: number }> = {};
  for (const [k, v] of Object.entries(map)) {
    out[k] = {
      maxSeq: Number(v.max_seq ?? v.maxSeq ?? 0) || 0,
      hasReadSeq: Number(v.has_read_seq ?? v.hasReadSeq ?? 0) || 0,
      maxSeqTime: Number(v.max_seq_time ?? v.maxSeqTime ?? 0) || 0,
    };
  }
  return out;
}

// 网关路径: GET /api/v1/messages/by-seq?conversation_id=&seqs=
export async function getMessagesBySeq(
  conversationId: string,
  seqs: number[]
): Promise<Message[]> {
  if (!seqs.length) return [];
  const q = `?conversation_id=${encodeURIComponent(conversationId)}&seqs=${seqs.join(",")}`;
  const res = await request<ApiResponse<Record<string, unknown>>>(`/messages/by-seq${q}`);
  const raw = res.data || {};
  const list = (raw.msg_data ?? raw.msgData ?? raw.messages ?? []) as Record<string, unknown>[];
  return list.map((item) => toMessage(item));
}

// ---------- 联系人 ----------
// 网关路径: GET /api/v1/relations/friends
// 后端返回 { friends: [{ friend_user_id, remark, is_pinned, ... }], total }
export async function getContacts(): Promise<Contact[]> {
  return getAllContacts();
}

/** PUT /api/v1/relations/friends/:friend_id — 局部更新备注 / 置顶 */
export async function updateFriend(
  friendId: string,
  patch: { remark?: string; isPinned?: boolean }
): Promise<void> {
  const body: Record<string, unknown> = {};
  if (patch.remark !== undefined) body.remark = patch.remark;
  if (patch.isPinned !== undefined) body.is_pinned = patch.isPinned;
  await request(`/relations/friends/${encodeURIComponent(friendId)}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

export type IncrementalFriendInfo = {
  friendUserId: string;
  remark: string;
  isPinned: boolean;
  nickname: string;
  avatarUrl: string;
  createTime?: number;
};

export type IncrementalFriendsResult = {
  version: number;
  versionId: string;
  full: boolean;
  delete: string[];
  insert: IncrementalFriendInfo[];
  update: IncrementalFriendInfo[];
  sortVersion: number;
};

function mapFriendInfo(raw: Record<string, unknown>): IncrementalFriendInfo {
  return {
    friendUserId: String(raw.friend_user_id ?? raw.friendUserId ?? "").trim(),
    remark: String(raw.remark ?? ""),
    isPinned: Boolean(raw.is_pinned ?? raw.isPinned),
    nickname: String(raw.nickname ?? ""),
    avatarUrl: String(raw.avatar_url ?? raw.avatarUrl ?? ""),
    createTime: Number(raw.create_time ?? raw.createTime ?? 0) || undefined,
  };
}

/** POST /api/v1/relations/friends/incremental */
export async function getIncrementalFriends(
  versionId: string,
  version: number
): Promise<IncrementalFriendsResult> {
  const res = await request<ApiResponse<Record<string, unknown>>>(
    "/relations/friends/incremental",
    {
      method: "POST",
      body: JSON.stringify({ version_id: versionId, version }),
    }
  );
  const d = res.data ?? {};
  const insertRaw = (d.insert ?? []) as Record<string, unknown>[];
  const updateRaw = (d.update ?? []) as Record<string, unknown>[];
  const deleteRaw = (d.delete ?? []) as unknown[];
  return {
    version: Number(d.version ?? 0),
    versionId: String(d.version_id ?? d.versionId ?? ""),
    full: Boolean(d.full),
    delete: deleteRaw.map((x) => String(x)).filter(Boolean),
    insert: insertRaw.map(mapFriendInfo).filter((f) => f.friendUserId),
    update: updateRaw.map(mapFriendInfo).filter((f) => f.friendUserId),
    sortVersion: Number(d.sort_version ?? d.sortVersion ?? 0),
  };
}

/** POST /api/v1/relations/friends/full-ids */
export async function getFullFriendUserIDs(): Promise<string[]> {
  const res = await request<
    ApiResponse<{ user_ids?: string[]; userIds?: string[] }>
  >("/relations/friends/full-ids", {
    method: "POST",
    body: JSON.stringify({}),
  });
  const ids = res.data?.user_ids ?? res.data?.userIds ?? [];
  return Array.isArray(ids) ? ids.map(String).filter(Boolean) : [];
}

function friendRowToContact(f: {
  friend_user_id?: string;
  friendUserId?: string;
  remark?: string;
  is_pinned?: boolean;
  isPinned?: boolean;
  nickname?: string;
  avatar_url?: string;
  avatarUrl?: string;
}): Contact | null {
  const id = String(f.friend_user_id ?? f.friendUserId ?? "").trim();
  if (!id) return null;
  const remark = String(f.remark ?? "");
  const nickname = String(f.nickname ?? "") || id;
  return {
    userId: id,
    displayName: remark || nickname,
    nickname,
    username: id,
    avatar: String(f.avatar_url ?? f.avatarUrl ?? ""),
    status: "offline",
    isFriend: true,
    remark: remark || undefined,
    isPinned: Boolean(f.is_pinned ?? f.isPinned),
  };
}

/** 分页拉全量好友（Full sync）；优先用响应内 nickname/avatar */
export async function getAllContacts(): Promise<Contact[]> {
  const pageSize = 100;
  let offset = 0;
  const all: Contact[] = [];
  for (;;) {
    const res = await request<
      ApiResponse<{
        friends?: Array<Record<string, unknown>>;
        total?: number;
      }>
    >(`/relations/friends?offset=${offset}&limit=${pageSize}`);
    const friends = res.data?.friends;
    if (!Array.isArray(friends) || friends.length === 0) break;
    for (const raw of friends) {
      const c = friendRowToContact(raw as Parameters<typeof friendRowToContact>[0]);
      if (c) all.push(c);
    }
    offset += friends.length;
    const total = Number(res.data?.total ?? 0);
    if (friends.length < pageSize || (total > 0 && offset >= total)) break;
  }

  // 若服务端未 join 昵称头像，用 batch 补齐
  const needEnrich = all.filter((c) => !c.nickname || c.nickname === c.userId || !c.avatar);
  if (needEnrich.length === 0) return all;
  try {
    const users = await getUsersBatch(needEnrich.map((c) => c.userId));
    const byId = new Map(users.map((u) => [u.userId, u]));
    return all.map((c) => {
      const u = byId.get(c.userId);
      if (!u) return c;
      const nickname = u.displayName || u.username || c.nickname || c.userId;
      return {
        ...c,
        nickname,
        username: u.username || c.username,
        avatar: u.avatar || c.avatar,
        displayName: c.remark || nickname,
      };
    });
  } catch {
    return all;
  }
}

// 网关路径: GET /api/v1/users/batch?ids=1,2,3
// 后端返回 { users: [...] }，需传入已认证身份
export async function getUsersBatch(userIds: string[]): Promise<User[]> {
  const params = `ids=${userIds.map(encodeURIComponent).join(",")}`;
  const res = await request<ApiResponse<{ users: Record<string, unknown>[] | Record<string, Record<string, unknown>> }>>(
    `/users/batch?${params}`
  );
  const users = res.data?.users;
  if (!users) return [];
  const list = Array.isArray(users) ? users : Object.values(users);
  return list.map((item) => toUser(item as Record<string, unknown>));
}

// 网关路径: GET /api/v1/users/search?keyword=
// 后端返回 { users: [...], total: N }
export async function searchUsers(query: string): Promise<User[]> {
  const res = await request<ApiResponse<{ users: Record<string, unknown>[] }>>(
    `/users/search?keyword=${encodeURIComponent(query)}`
  );
  const data = res.data;
  if (!data) return [];
  const list = (data as { users?: Record<string, unknown>[] }).users || [];
  return list.map((item) => toUser(item));
}

// ---------- 好友请求 ----------
// POST /api/v1/relations/friend-requests
// body: { to_user_id, message }
export async function sendFriendRequest(
  toUserId: string,
  message: string = ""
): Promise<void> {
  await request("/relations/friend-requests", {
    method: "POST",
    body: JSON.stringify({ to_user_id: toUserId, message }),
  });
}
// PUT /api/v1/relations/friend-requests/:id/respond
// body: { handle_result, handle_msg }
export async function respondFriendRequest(
  fromUserId: string,
  handleResult: number,
  handleMsg?: string
): Promise<void> {
  await request(`/relations/friend-requests/${fromUserId}/respond`, {
    method: "PUT",
    body: JSON.stringify({
      handle_result: handleResult,
      handle_msg: handleMsg || "",
    }),
  });
}
// GET /api/v1/relations/incoming-applies?handle_results=0&offset=0&limit=20
export async function getIncomingRequests(
  params?: { handleResults?: number[]; offset?: number; limit?: number }
): Promise<FriendRequest[]> {
  const query = new URLSearchParams();
  if (params?.handleResults?.length) {
    query.set("handle_results", params.handleResults.join(","));
  }
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  const res = await request<ApiResponse<Record<string, unknown>[] | { requests: unknown[] }>>(
    `/relations/incoming-applies?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { requests?: unknown[] }).requests || [];
  return enrichFriendRequests(list.map((item) => toFriendRequest(item as Record<string, unknown>)));
}

// GET /api/v1/relations/outgoing-applies?handle_results=0&offset=0&limit=20
export async function getOutgoingRequests(
  params?: { handleResults?: number[]; offset?: number; limit?: number }
): Promise<FriendRequest[]> {
  const query = new URLSearchParams();
  if (params?.handleResults?.length) {
    query.set("handle_results", params.handleResults.join(","));
  }
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  const res = await request<ApiResponse<Record<string, unknown>[] | { requests: unknown[] }>>(
    `/relations/outgoing-applies?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { requests?: unknown[] }).requests || [];
  return enrichFriendRequests(list.map((item) => toFriendRequest(item as Record<string, unknown>)));
}

// GET /api/v1/relations/unhandled-count
export async function getUnhandledRequestCount(): Promise<number> {
  const res = await request<ApiResponse<{ count: number }>>(
    "/relations/unhandled-count"
  );
  const count = res.data?.count ?? 0;
  return count;
}

// DELETE /api/v1/relations/friends/:friend_id
export async function deleteFriend(friendId: string): Promise<void> {
  await request(`/relations/friends/${friendId}`, {
    method: "DELETE",
  });
}

/** POST /api/v1/users/online-status — 批量查询在线状态 */
export async function getUsersOnlineStatus(
  userIds: string[]
): Promise<Array<{ userId: string; status: User["status"]; platformIds?: number[] }>> {
  const ids = [...new Set(userIds.filter(Boolean))];
  if (!ids.length) return [];
  const res = await request<
    ApiResponse<{
      statuses?: Array<{
        user_id?: string;
        userId?: string;
        status?: number;
        platform_ids?: number[];
        platformIds?: number[];
      }>;
    }>
  >("/users/online-status", {
    method: "POST",
    body: JSON.stringify({ user_ids: ids }),
  });
  const list = res.data?.statuses ?? [];
  return list.map((s) => ({
    userId: String(s.user_id ?? s.userId ?? ""),
    status: Number(s.status ?? 0) === 1 ? "online" : "offline",
    platformIds: s.platform_ids ?? s.platformIds,
  }));
}

// GET /api/v1/relations/blocks?offset=&limit=
export async function getBlackList(params?: {
  offset?: number;
  limit?: number;
}): Promise<BlacklistEntry[]> {
  const query = new URLSearchParams();
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  const qs = query.toString();
  const res = await request<
    ApiResponse<{
      blacks?: Array<{
        blocked_user_id?: string;
        blockedUserId?: string;
        create_time?: number | string;
        createTime?: number | string;
        add_source?: number;
        addSource?: number;
        operator_user_id?: string;
        operatorUserId?: string;
        ex?: string;
      }>;
      total?: number;
    }>
  >(`/relations/blocks${qs ? `?${qs}` : ""}`);

  const blacks = res.data?.blacks ?? [];
  if (blacks.length === 0) return [];

  const meta = blacks.map((b) => ({
    userId: String(b.blocked_user_id ?? b.blockedUserId ?? ""),
    createTime: Number(b.create_time ?? b.createTime ?? 0),
    addSource: Number(b.add_source ?? b.addSource ?? 0),
    operatorUserId: String(b.operator_user_id ?? b.operatorUserId ?? ""),
    ex: String(b.ex ?? ""),
  })).filter((b) => b.userId);

  try {
    const users = await getUsersBatch(meta.map((m) => m.userId));
    const byId = new Map(users.map((u) => [u.userId, u]));
    return meta.map((m) => {
      const u = byId.get(m.userId);
      return {
        userId: m.userId,
        displayName: u?.displayName || u?.username || m.userId,
        username: u?.username || m.userId,
        avatar: u?.avatar || "",
        createTime: m.createTime,
        addSource: m.addSource,
        operatorUserId: m.operatorUserId || undefined,
        ex: m.ex || undefined,
      };
    });
  } catch {
    return meta.map((m) => ({
      userId: m.userId,
      displayName: m.userId,
      username: m.userId,
      avatar: "",
      createTime: m.createTime,
      addSource: m.addSource,
      operatorUserId: m.operatorUserId || undefined,
      ex: m.ex || undefined,
    }));
  }
}

// POST /api/v1/relations/blocks
export async function blockUser(blockedUserId: string): Promise<void> {
  await request("/relations/blocks", {
    method: "POST",
    body: JSON.stringify({ blocked_user_id: blockedUserId }),
  });
}

// DELETE /api/v1/relations/blocks/:user_id
export async function unblockUser(userId: string): Promise<void> {
  await request(`/relations/blocks/${encodeURIComponent(userId)}`, {
    method: "DELETE",
  });
}

// 后端 FriendRequestInfo (proto) -> 前端 FriendRequest
function toFriendRequest(raw: Record<string, unknown>): FriendRequest {
  const statusNum = Number(raw.status ?? raw.handle_result ?? 0);
  const createdAtMs = Number(raw.created_at ?? raw.create_time ?? 0);
  const updatedAtMs = Number(raw.updated_at ?? raw.handle_time ?? 0);
  return {
    requestId: String(raw.request_id || (raw.from_user_id + "_" + raw.to_user_id)),
    fromUserId: String(raw.from_user_id ?? raw.fromUserId ?? ""),
    toUserId: String(raw.to_user_id ?? raw.toUserId ?? ""),
    message: String(raw.message ?? raw.req_msg ?? ""),
    status: statusNum === 1 ? "accepted" : statusNum === -1 ? "rejected" : "pending",
    createdAt: createdAtMs > 0 ? new Date(createdAtMs).toISOString() : "",
    updatedAt: updatedAtMs > 0 ? new Date(updatedAtMs).toISOString() : "",
  };
}

async function enrichFriendRequests(requests: FriendRequest[]): Promise<FriendRequest[]> {
  const ids = Array.from(new Set(requests.flatMap((req) => [req.fromUserId, req.toUserId]).filter(Boolean)));
  if (ids.length === 0) return requests;

  try {
    const users = await getUsersBatch(ids);
    const userMap = new Map(users.map((user) => [user.userId, user]));
    return requests.map((req) => ({
      ...req,
      fromUser: userMap.get(req.fromUserId),
      toUser: userMap.get(req.toUserId),
    }));
  } catch {
    return requests;
  }
}
// ---------- 群组 ----------
// 网关路径: POST /api/v1/groups
// 创建群组；会话由 group 服务侧事件联动生成（conversation_id = gid_<group_id>）
export async function createGroup(data: CreateGroupRequest): Promise<Group> {
  const res = await request<ApiResponse<Record<string, unknown>>>("/groups", {
    method: "POST",
    body: JSON.stringify({
      group_name: data.name,
      member_ids: data.memberIds,
      ...(data.avatar ? { face_url: data.avatar } : {}),
    }),
  });
  const raw = res.data || {};
  const groupRaw = (raw.group as Record<string, unknown> | undefined) || raw;
  return {
    groupId: String(raw.group_id ?? raw.groupId ?? groupRaw.group_id ?? groupRaw.groupId ?? ""),
    name: String(groupRaw.group_name ?? groupRaw.name ?? data.name),
    avatar: String(groupRaw.face_url ?? groupRaw.avatar ?? data.avatar ?? ""),
    ownerId: String(
      groupRaw.owner_user_id ?? groupRaw.ownerUserId ?? groupRaw.ownerId ?? groupRaw.creator_user_id ?? ""
    ),
    memberCount: Number(
      groupRaw.member_count ?? groupRaw.memberCount ?? data.memberIds.length + 1
    ),
    introduction: String(groupRaw.introduction ?? ""),
    notification: String(groupRaw.notification ?? ""),
    needVerification: Boolean(groupRaw.need_verification ?? false),
    createdAt: String(groupRaw.create_time ?? groupRaw.createdAt ?? new Date().toISOString()),
  };
}

// 网关路径: GET /api/v1/groups/joined
export async function getGroups(): Promise<Group[]> {
  return getAllJoinedGroups();
}

/** 分页拉全量已加入群（Full sync） */
export async function getAllJoinedGroups(): Promise<Group[]> {
  const pageSize = 100;
  let offset = 0;
  const all: Group[] = [];
  for (;;) {
    const res = await request<
      ApiResponse<{ groups?: unknown[]; total?: number } | unknown[]>
    >(`/groups/joined?offset=${offset}&limit=${pageSize}`);
    const data = res.data;
    if (!data) break;
    const list = Array.isArray(data)
      ? data
      : (data as { groups?: unknown[] }).groups || [];
    if (!list.length) break;
    for (const item of list) {
      all.push(mapGroupInfo(item as Record<string, unknown>));
    }
    offset += list.length;
    const total = Array.isArray(data) ? 0 : Number((data as { total?: number }).total ?? 0);
    if (list.length < pageSize || (total > 0 && offset >= total)) break;
  }
  return all;
}

export type IncrementalJoinGroupResult = {
  version: number;
  versionId: string;
  full: boolean;
  delete: string[];
  insert: Group[];
  update: Group[];
  sortVersion: number;
};

/** POST /api/v1/groups/joined/incremental */
export async function getIncrementalJoinGroup(
  versionId: string,
  version: number
): Promise<IncrementalJoinGroupResult> {
  const res = await request<ApiResponse<Record<string, unknown>>>(
    "/groups/joined/incremental",
    {
      method: "POST",
      body: JSON.stringify({ version_id: versionId, version }),
    }
  );
  const d = res.data ?? {};
  const insertRaw = (d.insert ?? []) as Record<string, unknown>[];
  const updateRaw = (d.update ?? []) as Record<string, unknown>[];
  const deleteRaw = (d.delete ?? []) as unknown[];
  return {
    version: Number(d.version ?? 0),
    versionId: String(d.version_id ?? d.versionId ?? ""),
    full: Boolean(d.full),
    delete: deleteRaw.map((x) => String(x)).filter(Boolean),
    insert: insertRaw.map((item) => mapGroupInfo(item)).filter((g) => g.groupId),
    update: updateRaw.map((item) => mapGroupInfo(item)).filter((g) => g.groupId),
    sortVersion: Number(d.sort_version ?? d.sortVersion ?? 0),
  };
}

/** POST /api/v1/groups/joined/full-ids */
export async function getFullJoinGroupIDs(): Promise<string[]> {
  const res = await request<
    ApiResponse<{ group_ids?: string[]; groupIds?: string[] }>
  >("/groups/joined/full-ids", {
    method: "POST",
    body: JSON.stringify({}),
  });
  const ids = res.data?.group_ids ?? res.data?.groupIds ?? [];
  return Array.isArray(ids) ? ids.map(String).filter(Boolean) : [];
}

export type IncrementalGroupMemberResult = {
  version: number;
  versionId: string;
  full: boolean;
  delete: string[];
  insert: GroupMemberInfo[];
  update: GroupMemberInfo[];
  group?: Group;
  sortVersion: number;
};

function mapGroupMemberInfo(
  item: Record<string, unknown>,
  fallbackGroupId = ""
): GroupMemberInfo {
  return {
    userId: String(item.user_id ?? item.userId ?? ""),
    groupId: String(item.group_id ?? item.groupId ?? fallbackGroupId),
    displayName: String(item.nickname ?? item.displayName ?? ""),
    username: String(item.nickname ?? item.username ?? ""),
    avatar: String(item.face_url ?? item.avatar ?? ""),
    roleLevel: Number(item.role_level ?? item.roleLevel ?? 0),
    muteEndTime: Number(item.mute_end_time ?? item.muteEndTime ?? 0),
    joinedAt: String(item.join_time ?? item.joinedAt ?? ""),
  };
}

/** POST /api/v1/groups/:id/members/incremental */
export async function getIncrementalGroupMember(
  groupId: string,
  versionId: string,
  version: number
): Promise<IncrementalGroupMemberResult> {
  const id = parseGroupId(groupId);
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/groups/${id}/members/incremental`,
    {
      method: "POST",
      body: JSON.stringify({ version_id: versionId, version }),
    }
  );
  const d = res.data ?? {};
  const insertRaw = (d.insert ?? []) as Record<string, unknown>[];
  const updateRaw = (d.update ?? []) as Record<string, unknown>[];
  const deleteRaw = (d.delete ?? []) as unknown[];
  const groupRaw = (d.group ?? null) as Record<string, unknown> | null;
  return {
    version: Number(d.version ?? 0),
    versionId: String(d.version_id ?? d.versionId ?? ""),
    full: Boolean(d.full),
    delete: deleteRaw.map((x) => String(x)).filter(Boolean),
    insert: insertRaw
      .map((item) => mapGroupMemberInfo(item, id))
      .filter((m) => m.userId),
    update: updateRaw
      .map((item) => mapGroupMemberInfo(item, id))
      .filter((m) => m.userId),
    group: groupRaw ? mapGroupInfo(groupRaw, id) : undefined,
    sortVersion: Number(d.sort_version ?? d.sortVersion ?? 0),
  };
}

/** POST /api/v1/groups/:id/members/full-ids */
export async function getFullGroupMemberUserIDs(
  groupId: string
): Promise<{ userIds: string[]; version: number; versionId: string }> {
  const id = parseGroupId(groupId);
  const res = await request<
    ApiResponse<{
      user_ids?: string[];
      userIds?: string[];
      version?: number;
      version_id?: string;
      versionId?: string;
    }>
  >(`/groups/${id}/members/full-ids`, {
    method: "POST",
    body: JSON.stringify({}),
  });
  const d = res.data ?? {};
  const ids = d.user_ids ?? d.userIds ?? [];
  return {
    userIds: Array.isArray(ids) ? ids.map(String).filter(Boolean) : [],
    version: Number(d.version ?? 0),
    versionId: String(d.version_id ?? d.versionId ?? ""),
  };
}

function mapGroupInfo(item: Record<string, unknown>, fallbackId = ""): Group {
  return {
    groupId: String(item.group_id ?? item.groupId ?? fallbackId),
    name: String(item.group_name ?? item.name ?? ""),
    avatar: String(item.face_url ?? item.avatar ?? ""),
    ownerId: String(
      item.owner_user_id ?? item.ownerUserId ?? item.ownerId ?? item.creator_user_id ?? ""
    ),
    memberCount: Number(item.member_count ?? item.memberCount ?? 0),
    introduction: String(item.introduction ?? ""),
    notification: String(item.notification ?? ""),
    needVerification: Boolean(
      item.need_verification ?? item.needVerification ?? false
    ),
    createdAt: String(item.create_time ?? item.createdAt ?? ""),
  };
}

// 网关路径: GET /api/v1/groups/:id
export async function getGroupInfo(groupId: string): Promise<Group> {
  const id = parseGroupId(groupId);
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/groups/${id}`
  );
  const raw = res.data || {};
  // GetGroupResp = { group: GroupInfo }
  const item = (raw.group as Record<string, unknown> | undefined) || raw;
  return mapGroupInfo(item, id);
}

// 网关路径: POST /api/v1/groups/info  body: { group_ids }
export async function getGroupsInfo(groupIds: string[]): Promise<Group[]> {
  const ids = Array.from(
    new Set(groupIds.map((id) => parseGroupId(id)).filter(Boolean))
  );
  if (ids.length === 0) return [];
  const res = await request<ApiResponse<Record<string, unknown> | { groups?: unknown[] }>>(
    "/groups/info",
    {
      method: "POST",
      body: JSON.stringify({ group_ids: ids }),
    }
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data as { groups?: unknown[] }).groups || [];
  return list.map((item) => mapGroupInfo(item as Record<string, unknown>));
}

// 网关路径: PUT /api/v1/groups/:id
export async function updateGroupInfo(data: UpdateGroupRequest): Promise<void> {
  const body: Record<string, unknown> = {};
  if (data.name !== undefined) body.group_name = data.name;
  if (data.avatar !== undefined) body.face_url = data.avatar;
  if (data.introduction !== undefined) body.introduction = data.introduction;
  if (data.notification !== undefined) body.notification = data.notification;
  if (data.needVerification !== undefined) body.need_verification = data.needVerification;
  await request(`/groups/${parseGroupId(data.groupId)}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

// 网关路径: DELETE /api/v1/groups/:id
export async function dismissGroup(groupId: string): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}`, { method: "DELETE" });
}

// 网关路径: PUT /api/v1/groups/:id/owner
export async function transferGroupOwner(groupId: string, newOwnerId: string): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/owner`, {
    method: "PUT",
    body: JSON.stringify({ new_owner_id: newOwnerId }),
  });
}

// 网关路径: POST /api/v1/groups/:id/members
export async function inviteToGroup(groupId: string, userIds: string[]): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/members`, {
    method: "POST",
    body: JSON.stringify({ member_ids: userIds }),
  });
}

// 网关路径: DELETE /api/v1/groups/:id/members/:userId
export async function kickGroupMember(groupId: string, userId: string): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/members/${userId}`, { method: "DELETE" });
}

// 网关路径: POST /api/v1/groups/:id/quit
export async function quitGroup(groupId: string): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/quit`, { method: "POST" });
}

// 网关路径: GET /api/v1/groups/:id/members?offset=&limit=
export async function getGroupMembers(
  groupId: string,
  params?: { offset?: number; limit?: number }
): Promise<GroupMemberInfo[]> {
  const id = parseGroupId(groupId);
  const query = new URLSearchParams();
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  else query.set("limit", "100");
  const res = await request<ApiResponse<Record<string, unknown>[] | { members: unknown[] }>>(
    `/groups/${id}/members?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { members?: unknown[] }).members || [];
  return list.map((item) =>
    mapGroupMemberInfo(item as Record<string, unknown>, id)
  );
}

/** Fetch all members by paging (used when incremental sync returns Full). */
export async function getAllGroupMembers(groupId: string): Promise<GroupMemberInfo[]> {
  const id = parseGroupId(groupId);
  const pageSize = 100;
  let offset = 0;
  const all: GroupMemberInfo[] = [];
  for (;;) {
    const page = await getGroupMembers(id, { offset, limit: pageSize });
    all.push(...page);
    if (page.length < pageSize) break;
    offset += pageSize;
  }
  return all;
}

// 网关路径: PUT /api/v1/groups/:id/mute
export async function setGroupMute(groupId: string, isMuted: boolean): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/mute`, {
    method: "PUT",
    body: JSON.stringify({ is_muted: isMuted }),
  });
}

// 网关路径: PUT /api/v1/groups/:id/members/:userId/mute
export async function setMemberMute(
  groupId: string,
  userId: string,
  muteEndTime: number
): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/members/${userId}/mute`, {
    method: "PUT",
    body: JSON.stringify({ mute_end_time: muteEndTime }),
  });
}

// ---------- 入群申请 ----------
// 网关路径: POST /api/v1/groups/:id/apply
export async function applyToJoinGroup(groupId: string, message?: string): Promise<void> {
  await request(`/groups/${parseGroupId(groupId)}/apply`, {
    method: "POST",
    body: JSON.stringify({ message: message || "" }),
  });
}

// 网关路径: GET /api/v1/groups/:id/applications
export async function getPendingApplications(groupId: string): Promise<GroupApplication[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { requests: unknown[] }>>(
    `/groups/${parseGroupId(groupId)}/applications`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { requests?: unknown[] }).requests || [];
  return enrichGroupApplications(list.map((item) => toGroupApplication(item as Record<string, unknown>)));
}

// 网关路径: GET /api/v1/groups/applications/mine
export async function getMyApplications(): Promise<GroupApplication[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { requests: unknown[] }>>(
    "/groups/applications/mine"
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { requests?: unknown[] }).requests || [];
  return list.map((item) => toGroupApplication(item as Record<string, unknown>));
}

// 网关路径: PUT /api/v1/groups/applications/:id
export async function handleApplication(
  application: Pick<GroupApplication, "groupId" | "userId">,
  accept: boolean,
  handleMsg?: string
): Promise<void> {
  await request(`/groups/applications/${application.userId}`, {
    method: "PUT",
    body: JSON.stringify({
      group_id: parseGroupId(application.groupId),
      user_id: application.userId,
      handle_result: accept ? 1 : -1,
      handled_msg: handleMsg || "",
    }),
  });
}

// 网关路径: GET /api/v1/groups/unhandled-application-count
export async function getUnhandledGroupApplicationCount(groupId: string): Promise<number> {
  try {
    const res = await request<ApiResponse<{ count: number }>>(
      `/groups/unhandled-application-count?group_id=${encodeURIComponent(parseGroupId(groupId))}`
    );
    return res.data?.count ?? 0;
  } catch {
    return 0;
  }
}

function toGroupApplication(raw: Record<string, unknown>): GroupApplication {
  const statusNum = Number(raw.status ?? raw.handle_result ?? 0);
  return {
    applicationId: String(raw.application_id ?? raw.request_id ?? `${raw.group_id ?? ""}_${raw.user_id ?? ""}`),
    groupId: String(raw.group_id ?? raw.groupId ?? ""),
    userId: String(raw.user_id ?? raw.userId ?? ""),
    groupName: String(raw.group_name ?? raw.groupName ?? ""),
    message: String(raw.req_msg ?? raw.message ?? ""),
    status: statusNum === 1 ? "accepted" : statusNum === -1 ? "rejected" : "pending",
    handleUserId: raw.handle_user_id ? String(raw.handle_user_id) : undefined,
    handleMsg: raw.handled_msg || raw.handle_msg ? String(raw.handled_msg ?? raw.handle_msg) : undefined,
    createdAt: toISOString(raw.req_time ?? raw.create_time ?? raw.created_at ?? raw.createdAt),
    updatedAt: toISOString(raw.handled_time ?? raw.updated_at ?? raw.handle_time ?? raw.updatedAt),
  };
}

async function enrichGroupApplications(applications: GroupApplication[]): Promise<GroupApplication[]> {
  const ids = Array.from(new Set(applications.map((item) => item.userId).filter(Boolean)));
  if (ids.length === 0) return applications;
  try {
    const users = await getUsersBatch(ids);
    const byID = new Map(users.map((user) => [user.userId, user]));
    return applications.map((item) => ({ ...item, user: byID.get(item.userId) }));
  } catch {
    return applications;
  }
}

function toISOString(value: unknown): string {
  if (value === undefined || value === null || value === "") return "";
  const numeric = Number(value);
  if (Number.isFinite(numeric) && numeric > 0) {
    const ms = numeric < 1e12 ? numeric * 1000 : numeric;
    const date = new Date(ms);
    return Number.isNaN(date.getTime()) ? "" : date.toISOString();
  }
  const date = new Date(String(value));
  return Number.isNaN(date.getTime()) ? "" : date.toISOString();
}

// ---------- 文件 ----------
type BackendFile = {
  file_id: string; name: string; content_type: string; size: number;
  sha256?: string; category: FileAttachment["category"]; expires_at: number;
};

function toAttachment(file: BackendFile): FileAttachment {
  return { fileId: file.file_id, name: file.name, contentType: file.content_type,
    size: Number(file.size), sha256: file.sha256, category: file.category,
    expiresAt: new Date(Number(file.expires_at)).toISOString() };
}

export async function updateCurrentUser(data: { nickname?: string; avatarUrl?: string }): Promise<User> {
  const body: Record<string, string> = {};
  if (data.nickname !== undefined) body.nickname = data.nickname;
  if (data.avatarUrl !== undefined) body.avatar_url = data.avatarUrl;
  await request("/users/me", { method: "PUT", body: JSON.stringify(body) });
  return getCurrentUser();
}

/** OpenIM: setGlobalRecvMessageOpt */
export async function setGlobalRecvMessageOpt(opt: 0 | 1 | 2): Promise<void> {
  await request("/users/me/global-recv-msg-opt", {
    method: "PUT",
    body: JSON.stringify({ global_recv_msg_opt: opt }),
  });
}

/** OpenIM: getGlobalRecvMessageOpt */
export async function getGlobalRecvMessageOpt(): Promise<0 | 1 | 2> {
  const res = await request<ApiResponse<Record<string, unknown>>>("/users/me/global-recv-msg-opt");
  const d = res.data || {};
  const value = Number(d.global_recv_msg_opt ?? d.globalRecvMsgOpt ?? 0);
  if (value === 1 || value === 2) return value;
  return 0;
}

async function sha256(file: File): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

function putFile(url: string, file: File, headers: Record<string, string>, onProgress?: (value: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    Object.entries(headers || {}).forEach(([key, value]) => xhr.setRequestHeader(key, value));
    xhr.upload.onprogress = (event) => event.lengthComputable && onProgress?.(Math.round((event.loaded / event.total) * 100));
    xhr.onload = () => xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`Upload failed (${xhr.status})`));
    xhr.onerror = () => reject(new Error("Upload connection failed"));
    xhr.send(file);
  });
}

export async function uploadFile(file: File, onProgress?: (value: number) => void): Promise<FileAttachment> {
  const hash = await sha256(file);
  const initiated = await request<ApiResponse<{ file: BackendFile; upload_url: string; headers: Record<string, string>; already_uploaded: boolean }>>("/files/initiate", {
    method: "POST",
    body: JSON.stringify({ name: file.name, content_type: file.type || "application/octet-stream", size: file.size, sha256: hash }),
  });
  if (!initiated.data.already_uploaded) {
    await putFile(initiated.data.upload_url, file, initiated.data.headers, onProgress);
    const completed = await request<ApiResponse<{ file: BackendFile }>>(`/files/${initiated.data.file.file_id}/complete`, { method: "POST" }, 60_000);
    return toAttachment(completed.data.file);
  }
  onProgress?.(100);
  return toAttachment(initiated.data.file);
}

export async function uploadAvatar(
  file: File,
  target: { type: "user" | "group"; id?: string },
  onProgress?: (value: number) => void
): Promise<string> {
  if (!["image/jpeg", "image/png", "image/webp"].includes(file.type)) {
    throw new Error("头像仅支持 JPEG、PNG 或 WebP 格式");
  }
  if (file.size > 5 * 1024 * 1024) throw new Error("头像不能超过 5 MiB");
  const hash = await sha256(file);
  const initiateEndpoint = target.type === "user"
    ? "/users/me/avatar/initiate"
    : `/groups/${parseGroupId(target.id || "")}/avatar/initiate`;
  const initiated = await request<ApiResponse<{ upload: { file: BackendFile; upload_url: string; headers: Record<string, string>; already_uploaded: boolean } }>>(initiateEndpoint, {
    method: "POST",
    body: JSON.stringify({ name: file.name, content_type: file.type, size: file.size, sha256: hash }),
  });
  const upload = initiated.data.upload;
  if (!upload?.file?.file_id) throw new Error("头像上传初始化响应无效");
  if (!upload.already_uploaded && !upload.upload_url) throw new Error("头像上传地址为空");
  if (!upload.already_uploaded) {
    await putFile(upload.upload_url, file, upload.headers, onProgress);
  }
  const completeEndpoint = target.type === "user"
    ? `/users/me/avatar/${upload.file.file_id}/complete`
    : `/groups/${parseGroupId(target.id || "")}/avatar/${upload.file.file_id}/complete`;
  const completed = await request<ApiResponse<{ avatar_url?: string; avatarUrl?: string }>>(
    completeEndpoint,
    { method: "POST" },
    60_000
  );
  onProgress?.(100);
  const avatarURL = String(completed.data?.avatar_url ?? completed.data?.avatarUrl ?? "");
  if (!avatarURL) throw new Error("头像上传完成但未返回地址");
  return avatarURL;
}

export async function resolveAvatarURL(source: string): Promise<string> {
  if (!source) return "";
  if (source.startsWith("blob:") || source.startsWith("data:")) return source;
  const match = source.match(/\/files\/([^/]+)\/avatar(?:$|\?)/);
  if (!match) return source;
  const res = await request<ApiResponse<{ download_url?: string; downloadUrl?: string }>>(
    `/files/${match[1]}/avatar`
  );
  const url = String(res.data?.download_url ?? res.data?.downloadUrl ?? "");
  if (!url) throw new Error("头像下载地址为空");
  return url;
}

export async function getFileDownloadURL(fileId: string): Promise<string> {
  const res = await request<ApiResponse<{ download_url: string }>>(`/files/${fileId}/download`);
  return res.data.download_url;
}
