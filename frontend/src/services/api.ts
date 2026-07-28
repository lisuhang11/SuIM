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
  FriendRequest,
} from "@/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:9000/api/v1";

// ---------- 通用请求 ----------
const ERROR_ZH: Record<string, string> = {
  // 认证
  "missing or invalid authorization header": "缺少或无效的认证头",
  "empty token": "令牌为空",
  "invalid or expired token": "令牌无效或已过期",
  "token missing user identity": "令牌缺少用户标识",
  "not authenticated": "未登录",
  // 用户
  "user not found": "用户不存在",
  "user already exists": "该用户已存在",
  "invalid password": "密码错误",
  "password must be 8-32 characters with at least one letter and one number": "密码需8-32位，包含字母和数字",
  "account is disabled": "账号已被禁用",
  "invalid email format": "邮箱格式不正确",
  "username, email and password are required": "用户名、邮箱和密码为必填项",
  "email and password are required": "邮箱和密码为必填项",
  "failed to create user": "创建用户失败",
  // 通用
  "internal server error": "服务器内部错误",
  "too many requests": "请求过于频繁，请稍后再试",
};
function translateError(msg: string): string {
  return ERROR_ZH[msg] || msg;
}

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
    const rawMsg = err.message || `HTTP ${res.status}`;
    throw new Error(translateError(rawMsg));
  }

  return res.json();
}

// ---------- 数据类型转换 ----------
// 后端 proto UserInfo → 前端 User
// UserInfo: user_id, nickname, email, avatar_url, create_time, updated_at
// RegisterReq.username 被存储为 UserInfo.nickname（显示昵称）
// email 即登录标识，无独立 username 字段
function toUser(info: Record<string, unknown>): User {
  const userId = String(info.user_id ?? "");
  const email = String(info.email ?? "");
  const nickname = String(info.nickname ?? "");
  return {
    userId,
    suid: userId,
    username: email,               // email 作为登录标识
    displayName: nickname || email.split("@")[0] || userId,
    avatar: String(info.avatar_url ?? ""),
    email,
    status: "offline",
    lastSeen: undefined,
    createdAt: info.create_time && Number(info.create_time) > 0
      ? new Date(Number(info.create_time)).toISOString()
      : info.updated_at && Number(info.updated_at) > 0
        ? new Date(Number(info.updated_at)).toISOString()
        : new Date().toISOString(),
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
    lastMessage: (raw.last_message || raw.lastMessage || undefined) as Message | undefined,
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
  return toUser((d.user ?? d) as Record<string, unknown>);
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
// GetFriendsResp: { friend_ids: string[], total: int32 }
export async function getContacts(): Promise<Contact[]> {
  const res = await request<ApiResponse<{ friend_ids: string[]; total: number }>>(
    "/relations/friends"
  );
  const data = res.data;
  if (!data?.friend_ids || data.friend_ids.length === 0) return [];
  // 批量查询用户信息以获取昵称等
  try {
    const usersRes = await request<ApiResponse<{ users: Record<string, Record<string, unknown>> }>>(
      `/users/batch?ids=${data.friend_ids.join(",")}`
    );
    const users = usersRes.data?.users || {};
    return data.friend_ids.map((id) => {
      const info = users[id] || {};
      return {
        userId: id,
        displayName: String(info.nickname ?? id),
        username: String(info.email ?? id),
        avatar: String(info.avatar_url ?? ""),
        status: "offline" as const,
        isFriend: true,
      };
    });
  } catch {
    // batch 查询失败，用 ID 作为显示名
    return data.friend_ids.map((id) => ({
      userId: id,
      displayName: id,
      username: id,
      avatar: "",
      status: "offline" as const,
      isFriend: true,
    }));
  }
}

// 网关路径: GET /api/v1/users/search?keyword=
// SearchUsersResp: { users: UserInfo[] }
export async function searchUsers(query: string): Promise<User[]> {
  const res = await request<ApiResponse<{ users: Record<string, unknown>[] }>>(
    `/users/search?keyword=${encodeURIComponent(query)}`
  );
  const data = res.data;
  if (!data?.users || !Array.isArray(data.users)) return [];
  return data.users.map((item) => toUser(item));
}

// ---------- 好友关系 ----------
// 发送好友请求 POST /api/v1/relations/friend-requests
// SendFriendRequestReq: { from_user_id, to_user_id, message }
export async function sendFriendRequest(
  fromUserId: string,
  toUserId: string,
  message: string = ""
): Promise<void> {
  await request("/relations/friend-requests", {
    method: "POST",
    body: JSON.stringify({ from_user_id: fromUserId, to_user_id: toUserId, message }),
  });
}

// 响应好友请求 PUT /api/v1/relations/friend-requests/:id/respond
export async function respondFriendRequest(
  fromUserId: string,
  toUserId: string,
  handleResult: 1 | -1,
  handleMsg?: string
): Promise<void> {
  await request(`/relations/friend-requests/${fromUserId}/respond`, {
    method: "PUT",
    body: JSON.stringify({
      from_user_id: fromUserId,
      to_user_id: toUserId,
      handle_result: handleResult,
      handle_msg: handleMsg || "",
    }),
  });
}

// 获取收到的好友请求 GET /api/v1/relations/incoming-applies
// GetIncomingApplyToResp: { requests: FriendRequestInfo[], total: int32 }
// FriendRequestInfo: { from_user_id, to_user_id, message, handle_msg, status, created_at, updated_at }
export async function getIncomingFriendRequests(
  limit: number = 20,
  offset: number = 0
): Promise<FriendRequest[]> {
  const res = await request<ApiResponse<{ requests: Record<string, unknown>[]; total: number }>>(
    `/relations/incoming-applies?limit=${limit}&offset=${offset}&handle_results=0`
  );
  const data = res.data;
  if (!data?.requests) return [];
  return data.requests.map((item) => ({
    requestId: `${item.from_user_id ?? ""}_${item.to_user_id ?? ""}`,
    fromUserId: String(item.from_user_id ?? ""),
    fromUsername: "",
    fromDisplayName: String(item.from_user_id ?? ""), // 暂时用 ID，后续可通过 batch 查
    fromAvatar: "",
    toUserId: String(item.to_user_id ?? ""),
    message: String(item.message ?? ""),
    status: (
      Number(item.status) === 1 ? "accepted" :
      Number(item.status) === -1 ? "declined" :
      "pending"
    ) as FriendRequest["status"],
    createdAt: item.created_at && Number(item.created_at) > 0
      ? new Date(Number(item.created_at)).toISOString()
      : new Date().toISOString(),
  }));
}

// 获取发出的好友请求 GET /api/v1/relations/outgoing-applies
export async function getOutgoingRequests(
  limit: number = 20,
  offset: number = 0,
  handleResults: number[] = [0]
): Promise<{ toUserId: string; toNickname: string; message: string; status: number; createdAt: string }[]> {
  const handleResultParam = handleResults.map((r) => `handle_results=${r}`).join("&");
  const res = await request<ApiResponse<{ requests?: unknown[]; total?: number }>>(
    `/relations/outgoing-applies?limit=${limit}&offset=${offset}&${handleResultParam}`
  );
  const data = res.data;
  if (!data?.requests) return [];
  return (data.requests as Record<string, unknown>[]).map((item) => ({
    toUserId: String(item.to_user_id ?? ""),
    toNickname: String(item.nickname ?? item.to_nickname ?? ""),
    message: String(item.message ?? ""),
    status: Number(item.status ?? 0),
    createdAt: String(item.created_at ?? item.create_time ?? ""),
  }));
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
  return (list as Record<string, unknown>[]).map((item: Record<string, unknown>) => ({
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
