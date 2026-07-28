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
  FriendRequest,
  CreateGroupRequest,
  Group,
  GroupMemberInfo,
  GroupApplication,
  UpdateGroupRequest,
} from "@/types";

// 开发/联调时通过 Next.js rewrites 代理到 Docker 后端，避免跨域和网络不通问题
const API_BASE = process.env.NEXT_PUBLIC_API_URL || "/api/v1";
const REQUEST_TIMEOUT = 10_000; // 10 秒超时

// ---------- 通用请求（带超时） ----------
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

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT);

  const fullUrl = `${API_BASE}${endpoint}`;
  console.log(`[API] ${options.method || "GET"} ${fullUrl}`);

  try {
    const res = await fetch(fullUrl, {
      ...options,
      headers,
      signal: controller.signal,
    });

    if (!res.ok) {
      const err = await res.json().catch(() => ({ message: res.statusText }));
      console.error(`[API] ${options.method || "GET"} ${fullUrl} → ${res.status}`, err);
      // 401 时清除本地认证状态，下次刷新页面会跳转登录
      if (res.status === 401) {
        if (typeof window !== "undefined") {
          localStorage.removeItem("suim_token");
          localStorage.removeItem("suim_user");
        }
      }
      throw new Error(err.message || `HTTP ${res.status}`);
    }

    console.log(`[API] ${options.method || "GET"} ${fullUrl} → ${res.status} OK`);
    return res.json();
  } catch (err) {
    console.error(`[API] ${options.method || "GET"} ${fullUrl} → FAILED`, err);
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
// 注意: 后端 conversation_type: 1 = 私聊(single), 2 = 群聊(group)
function toConversation(raw: Record<string, unknown>): Conversation {
  const rawType = raw.conversation_type !== undefined ? Number(raw.conversation_type) : undefined;
  return {
    conversationId: String(raw.conversation_id ?? raw.conversationId ?? ""),
    type: (rawType !== undefined
      ? (rawType === 2 ? "group" : "private")  // 1=single, 2=group
      : String(raw.type || "private")) as Conversation["type"],
    title: String(raw.title || raw.group_name || ""),
    avatar: String(raw.face_url ?? raw.avatar ?? ""),
    unreadCount: Number(raw.unread_count ?? raw.unreadCount ?? 0),
    members: Array.isArray(raw.members) ? raw.members : [],
    isPinned: Boolean(raw.is_pinned ?? raw.isPinned ?? false),
    isMuted: Boolean(raw.is_muted ?? raw.isMuted ?? false),
    lastMessage: (raw.last_message && typeof raw.last_message === "object" ? raw.last_message : raw.lastMessage && typeof raw.lastMessage === "object" ? raw.lastMessage : undefined) as Message | undefined,
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
  return toUser((d.user as Record<string, unknown>) ?? d);
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
// 后端返回 { friend_ids: [...], total: N }，需要批量获取用户信息
export async function getContacts(): Promise<Contact[]> {
  const res = await request<ApiResponse<{ friend_ids?: string[] }>>(
    "/relations/friends"
  );
  const friendIds = res.data?.friend_ids;
  if (!friendIds || friendIds.length === 0) return [];

  // 批量获取用户信息
  try {
    const users = await getUsersBatch(friendIds);
    return users.map((u) => ({
      userId: u.userId,
      displayName: u.displayName,
      username: u.username,
      avatar: u.avatar,
      status: "offline" as const,
      isFriend: true,
    }));
  } catch {
    // 回退：仅返回 ID
    return friendIds.map((id) => ({
      userId: id,
      displayName: id,
      username: id,
      avatar: "",
      status: "offline" as const,
      isFriend: true,
    }));
  }
}

// 网关路径: GET /api/v1/users/batch?ids=1,2,3
// 后端返回 { users: [...] }，需传入已认证身份
async function getUsersBatch(userIds: string[]): Promise<User[]> {
  const params = userIds.map((id) => `ids=${encodeURIComponent(id)}`).join("&");
  const res = await request<ApiResponse<{ users: Record<string, unknown>[] }>>(
    `/users/batch?${params}`
  );
  const users = res.data?.users;
  if (!users) return [];
  return users.map((item) => toUser(item));
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
  return list.map((item) => ({
    groupId: String((item as Record<string, unknown>).group_id ?? (item as Record<string, unknown>).groupId ?? ""),
    name: String((item as Record<string, unknown>).group_name ?? (item as Record<string, unknown>).name ?? ""),
    avatar: String((item as Record<string, unknown>).face_url ?? (item as Record<string, unknown>).avatar ?? ""),
    ownerId: String((item as Record<string, unknown>).creator_user_id ?? (item as Record<string, unknown>).ownerId ?? ""),
    memberCount: Number((item as Record<string, unknown>).member_count ?? (item as Record<string, unknown>).memberCount ?? 0),
    introduction: String((item as Record<string, unknown>).introduction ?? ""),
    notification: String((item as Record<string, unknown>).notification ?? ""),
    needVerification: Boolean((item as Record<string, unknown>).need_verification ?? false),
    createdAt: String((item as Record<string, unknown>).create_time ?? (item as Record<string, unknown>).createdAt ?? ""),
  }));
}

// 网关路径: GET /api/v1/groups/:id
export async function getGroupInfo(groupId: string): Promise<Group> {
  const res = await request<ApiResponse<Record<string, unknown>>>(
    `/groups/${groupId}`
  );
  const item = res.data || {};
  return {
    groupId: String(item.group_id ?? item.groupId ?? groupId),
    name: String(item.group_name ?? item.name ?? ""),
    avatar: String(item.face_url ?? item.avatar ?? ""),
    ownerId: String(item.creator_user_id ?? item.ownerId ?? ""),
    memberCount: Number(item.member_count ?? item.memberCount ?? 0),
    introduction: String(item.introduction ?? ""),
    notification: String(item.notification ?? ""),
    needVerification: Boolean(item.need_verification ?? false),
    createdAt: String(item.create_time ?? item.createdAt ?? ""),
  };
}

// 网关路径: PUT /api/v1/groups/:id
export async function updateGroupInfo(data: UpdateGroupRequest): Promise<void> {
  const body: Record<string, unknown> = {};
  if (data.name !== undefined) body.group_name = data.name;
  if (data.avatar !== undefined) body.face_url = data.avatar;
  if (data.introduction !== undefined) body.introduction = data.introduction;
  if (data.notification !== undefined) body.notification = data.notification;
  if (data.needVerification !== undefined) body.need_verification = data.needVerification;
  await request(`/groups/${data.groupId}`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
}

// 网关路径: DELETE /api/v1/groups/:id
export async function dismissGroup(groupId: string): Promise<void> {
  await request(`/groups/${groupId}`, { method: "DELETE" });
}

// 网关路径: PUT /api/v1/groups/:id/owner
export async function transferGroupOwner(groupId: string, newOwnerId: string): Promise<void> {
  await request(`/groups/${groupId}/owner`, {
    method: "PUT",
    body: JSON.stringify({ new_owner_id: newOwnerId }),
  });
}

// 网关路径: POST /api/v1/groups/:id/members
export async function inviteToGroup(groupId: string, userIds: string[]): Promise<void> {
  await request(`/groups/${groupId}/members`, {
    method: "POST",
    body: JSON.stringify({ member_ids: userIds }),
  });
}

// 网关路径: DELETE /api/v1/groups/:id/members/:userId
export async function kickGroupMember(groupId: string, userId: string): Promise<void> {
  await request(`/groups/${groupId}/members/${userId}`, { method: "DELETE" });
}

// 网关路径: POST /api/v1/groups/:id/quit
export async function quitGroup(groupId: string): Promise<void> {
  await request(`/groups/${groupId}/quit`, { method: "POST" });
}

// 网关路径: GET /api/v1/groups/:id/members?offset=&limit=
export async function getGroupMembers(
  groupId: string,
  params?: { offset?: number; limit?: number }
): Promise<GroupMemberInfo[]> {
  const query = new URLSearchParams();
  if (params?.offset !== undefined) query.set("offset", String(params.offset));
  if (params?.limit !== undefined) query.set("limit", String(params.limit));
  else query.set("limit", "100");
  const res = await request<ApiResponse<Record<string, unknown>[] | { members: unknown[] }>>(
    `/groups/${groupId}/members?${query.toString()}`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { members?: unknown[] }).members || [];
  return list.map((item) => ({
    userId: String((item as Record<string, unknown>).user_id ?? (item as Record<string, unknown>).userId ?? ""),
    groupId: String((item as Record<string, unknown>).group_id ?? (item as Record<string, unknown>).groupId ?? groupId),
    displayName: String((item as Record<string, unknown>).nickname ?? (item as Record<string, unknown>).displayName ?? ""),
    username: String((item as Record<string, unknown>).nickname ?? (item as Record<string, unknown>).username ?? ""),
    avatar: String((item as Record<string, unknown>).face_url ?? (item as Record<string, unknown>).avatar ?? ""),
    roleLevel: Number((item as Record<string, unknown>).role_level ?? (item as Record<string, unknown>).roleLevel ?? 0),
    muteEndTime: Number((item as Record<string, unknown>).mute_end_time ?? (item as Record<string, unknown>).muteEndTime ?? 0),
    joinedAt: String((item as Record<string, unknown>).join_time ?? (item as Record<string, unknown>).joinedAt ?? ""),
  }));
}

// 网关路径: PUT /api/v1/groups/:id/mute
export async function setGroupMute(groupId: string, isMuted: boolean): Promise<void> {
  await request(`/groups/${groupId}/mute`, {
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
  await request(`/groups/${groupId}/members/${userId}/mute`, {
    method: "PUT",
    body: JSON.stringify({ mute_end_time: muteEndTime }),
  });
}

// ---------- 入群申请 ----------
// 网关路径: POST /api/v1/groups/:id/apply
export async function applyToJoinGroup(groupId: string, message?: string): Promise<void> {
  await request(`/groups/${groupId}/apply`, {
    method: "POST",
    body: JSON.stringify({ message: message || "" }),
  });
}

// 网关路径: GET /api/v1/groups/:id/applications
export async function getPendingApplications(groupId: string): Promise<GroupApplication[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { applications: unknown[] }>>(
    `/groups/${groupId}/applications`
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { applications?: unknown[] }).applications || [];
  return list.map((item) => toGroupApplication(item as Record<string, unknown>));
}

// 网关路径: GET /api/v1/groups/applications/mine
export async function getMyApplications(): Promise<GroupApplication[]> {
  const res = await request<ApiResponse<Record<string, unknown>[] | { applications: unknown[] }>>(
    "/groups/applications/mine"
  );
  const data = res.data;
  if (!data) return [];
  const list = Array.isArray(data) ? data : (data as { applications?: unknown[] }).applications || [];
  return list.map((item) => toGroupApplication(item as Record<string, unknown>));
}

// 网关路径: PUT /api/v1/groups/applications/:id
export async function handleApplication(
  applicationId: string,
  accept: boolean,
  handleMsg?: string
): Promise<void> {
  await request(`/groups/applications/${applicationId}`, {
    method: "PUT",
    body: JSON.stringify({
      handle_result: accept ? 1 : -1,
      handle_msg: handleMsg || "",
    }),
  });
}

// 网关路径: GET /api/v1/groups/unhandled-application-count
export async function getUnhandledGroupApplicationCount(): Promise<number> {
  try {
    const res = await request<ApiResponse<{ count: number }>>(
      "/groups/unhandled-application-count"
    );
    return res.data?.count ?? 0;
  } catch {
    return 0;
  }
}

function toGroupApplication(raw: Record<string, unknown>): GroupApplication {
  const statusNum = Number(raw.status ?? raw.handle_result ?? 0);
  return {
    applicationId: String(raw.application_id ?? raw.request_id ?? raw.applicationId ?? ""),
    groupId: String(raw.group_id ?? raw.groupId ?? ""),
    userId: String(raw.user_id ?? raw.userId ?? ""),
    groupName: String(raw.group_name ?? raw.groupName ?? ""),
    message: String(raw.req_msg ?? raw.message ?? ""),
    status: statusNum === 1 ? "accepted" : statusNum === -1 ? "rejected" : "pending",
    handleUserId: raw.handle_user_id ? String(raw.handle_user_id) : undefined,
    handleMsg: raw.handle_msg ? String(raw.handle_msg) : undefined,
    createdAt: String(raw.create_time ?? raw.created_at ?? raw.createdAt ?? ""),
    updatedAt: String(raw.updated_at ?? raw.handle_time ?? raw.updatedAt ?? ""),
  };
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
