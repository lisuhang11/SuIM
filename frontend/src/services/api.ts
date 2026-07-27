// ============================================================
// REST API 客户端 — 对接 SuIM API Gateway (port 9000)
// ============================================================
import { getToken } from "./storage";
import type {
  ApiResponse,
  AuthResponse,
  LoginRequest,
  RegisterRequest,
  User,
  Conversation,
  Message,
  Contact,
  CreateGroupRequest,
  Group,
} from "@/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:9000/api/v1";

// ---------- 通用请求 ----------
async function request<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw new Error(err.message || `HTTP ${res.status}`);
  }

  return res.json();
}

// ---------- 数据类型转换 ----------
// 后端 UserInfo -> 前端 User
function toUser(info: Record<string, unknown>): User {
  return {
    userId: String(info.user_id ?? info.userId ?? ""),
    username: String(info.nickname ?? info.username ?? ""),
    displayName: String(info.nickname ?? info.displayName ?? ""),
    avatar: String(info.avatar_url ?? info.avatar ?? ""),
    email: String(info.email ?? ""),
    status: (info.status as User["status"]) || "offline",
    lastSeen: info.last_seen ? String(info.last_seen) : undefined,
    createdAt: info.create_time
      ? new Date(Number(info.create_time)).toISOString()
      : String(info.updated_at ?? info.createdAt ?? new Date().toISOString()),
  };
}

// 后端会话响应 -> 前端 Conversation
function toConversation(raw: Record<string, unknown>): Conversation {
  return {
    conversationId: String(raw.conversation_id ?? raw.conversationId ?? ""),
    type: (raw.conversation_type !== undefined
      ? (Number(raw.conversation_type) === 1 ? "group" : "private")
      : String(raw.type || "private")) as Conversation["type"],
    title: String(raw.title || raw.group_name || ""),
    avatar: String(raw.face_url ?? raw.avatar ?? ""),
    unreadCount: Number(raw.unread_count ?? raw.unreadCount ?? 0),
    members: Array.isArray(raw.members) ? raw.members : [],
    isPinned: Boolean(raw.is_pinned ?? raw.isPinned ?? false),
    isMuted: Boolean(raw.is_muted ?? raw.isMuted ?? false),
    lastMessage: raw.last_message || raw.lastMessage || undefined,
    createdAt: String(raw.create_time ?? raw.createdAt ?? ""),
    updatedAt: String(raw.updated_at ?? raw.updatedAt ?? ""),
  };
}

// 后端消息响应 -> 前端 Message
function toMessage(raw: Record<string, unknown>): Message {
  return {
    messageId: String(raw.server_msg_id ?? raw.messageId ?? ""),
    conversationId: String(raw.conversation_id ?? raw.conversationId ?? ""),
    senderId: String(raw.send_id ?? raw.senderId ?? ""),
    senderName: String(raw.sender_nickname ?? raw.senderName ?? ""),
    senderAvatar: String(raw.sender_face_url ?? raw.senderAvatar ?? ""),
    content: String(raw.content ?? ""),
    type: (Number(raw.content_type) === 1 ? "image" : "text") as Message["type"],
    status: (raw.is_read ? "read" : "delivered") as Message["status"],
    createdAt: raw.send_time
      ? new Date(Number(raw.send_time)).toISOString()
      : String(raw.createdAt ?? new Date().toISOString()),
  };
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
  // backend LoginResp: { success, message, user, access_token, refresh_token }
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

// 网关路径: GET /api/v1/users/me（从 JWT 上下文获取当前用户）
export async function getCurrentUser(): Promise<User> {
  const res = await request<ApiResponse<Record<string, unknown>>>("/users/me");
  const d = res.data || ({} as Record<string, unknown>);
  return toUser(d.user ?? d);
}

// 网关路径: POST /api/v1/users/logout
export async function logout(): Promise<void> {
  await request("/users/logout", { method: "POST" });
}

// ---------- 会话 ----------
// 网关路径: GET /api/v1/conversations/owner
export async function getConversations(): Promise<Conversation[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { conversations: unknown[] }>>(
    "/conversations/owner"
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data as { conversations?: unknown[] }).conversations || [];
  return list.map((item) => toConversation(item as Record<string, unknown>));
}

// 网关路径: GET /api/v1/conversations/:id?owner_user_id=...
export async function getConversation(id: string): Promise<Conversation> {
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/conversations/${id}`
  );
  return toConversation(res.data || {});
}

// 网关路径: POST /api/v1/conversations/single
export async function createPrivateConversation(
  userId: string
): Promise<Conversation> {
  const res = await request<ApiResponse<Record<string, unknown>>>("/conversations/single", {
    method: "POST",
    body: JSON.stringify({ recv_id: userId }),
  });
  return toConversation(res.data || {});
}

// 网关路径: POST /api/v1/conversations/group
export async function createGroupConversation(
  data: CreateGroupRequest
): Promise<Conversation> {
  const res = await request<ApiResponse<Record<string, unknown>>>("/conversations/group", {
    method: "POST",
    body: JSON.stringify({
      group_name: data.name,
      member_ids: data.memberIds,
    }),
  });
  return toConversation(res.data || {});
}

// 网关路径: DELETE /api/v1/conversations/batch
export async function deleteConversation(id: string): Promise<void> {
  await request("/conversations/batch", {
    method: "DELETE",
    body: JSON.stringify({ conversation_ids: [id] }),
  });
}

// 网关路径: POST /api/v1/messages/read
export async function markAsRead(conversationId: string): Promise<void> {
  await request("/messages/read", {
    method: "POST",
    body: JSON.stringify({ conversation_id: conversationId }),
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
  const res = await request<ApiResponse<Record<string, unknown>[] | { messages: unknown[] }>>(
    `/messages/history?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data as { messages?: unknown[] }).messages || [];
  return list.map((item) => toMessage(item as Record<string, unknown>));
}

// ---------- 联系人 ----------
// 网关路径: GET /api/v1/relations/friends
export async function getContacts(): Promise<Contact[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { friends: unknown[] }>>(
    "/relations/friends"
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data as { friends?: unknown[] }).friends || [];
  return list.map((item: Record<string, unknown>) => ({
    userId: String(item.user_id ?? item.userId ?? ""),
    displayName: String(item.nickname ?? item.displayName ?? ""),
    username: String(item.nickname ?? item.username ?? ""),
    avatar: String(item.avatar_url ?? item.avatar ?? ""),
    status: "offline" as const,
    isFriend: true,
  }));
}

// 网关路径: GET /api/v1/users/search?keyword=
export async function searchUsers(query: string): Promise<User[]> {
  const res = await request<ApiResponse<Record<string, unknown>[]>>(
    `/users/search?keyword=${encodeURIComponent(query)}`
  );
  const data = res.data;
  if (!data || !Array.isArray(data)) return [];
  return data.map((item) => toUser(item));
}

// ---------- 群组 ----------
// 网关路径: GET /api/v1/groups/joined
export async function getGroups(): Promise<Group[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { groups: unknown[] }>>(
    "/groups/joined"
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data)
    ? data
    : (data as { groups?: unknown[] }).groups || [];
  return list.map((item: Record<string, unknown>) => ({
    groupId: String(item.group_id ?? item.groupId ?? ""),
    name: String(item.group_name ?? item.name ?? ""),
    avatar: String(item.face_url ?? item.avatar ?? ""),
    ownerId: String(item.creator_user_id ?? item.ownerId ?? ""),
    memberCount: Number(item.member_count ?? item.memberCount ?? 0),
    createdAt: String(item.create_time ?? item.createdAt ?? ""),
  }));
}

// ---------- 上传 ----------
export async function uploadFile(file: File): Promise<{ url: string }> {
  const formData = new FormData();
  formData.append("file", file);
  const token = getToken();

  const res = await fetch(`${API_BASE}/upload`, {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: formData,
  });

  if (!res.ok) {
    throw new Error("Upload failed");
  }

  const json = await res.json() as ApiResponse<{ url: string }>;
  return json.data;
}
