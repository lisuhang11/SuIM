// ============================================================
// SuIM 即时通讯系统 — 前端类型定义
// ============================================================

// ---------- 用户 ----------
export type UserStatus = "online" | "offline" | "away" | "busy";

export interface User {
  userId: string;
  username: string;
  displayName: string;
  avatar: string;
  email: string;
  status: UserStatus;
  lastSeen?: string; // ISO 8601
  createdAt: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  displayName: string;
  email: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  expiresAt: string;
  user: User;
}

// ---------- 会话 ----------
export type ConversationType = "private" | "group";

export type MemberRole = "owner" | "admin" | "member";

export interface ConversationMember {
  userId: string;
  conversationId: string;
  role: MemberRole;
  joinedAt: string;
  nickname?: string;
}

export interface Conversation {
  conversationId: string;
  type: ConversationType;
  title: string; // 群聊名称 / 私聊对方 displayName
  avatar: string;
  lastMessage?: Message;
  unreadCount: number;
  members: ConversationMember[];
  isPinned: boolean;
  isMuted: boolean;
  createdAt: string;
  updatedAt: string;
}

// ---------- 消息 ----------
export type MessageType = "text" | "image" | "file" | "system";
export type MessageStatus = "sending" | "sent" | "delivered" | "read" | "failed";

export interface FileAttachment {
  fileId: string;
  name: string;
  contentType: string;
  size: number;
  sha256?: string;
  category: "image" | "video" | "audio" | "document" | "other";
  expiresAt: string;
}

export interface Message {
  messageId: string;
  conversationId: string;
  senderId: string;
  senderName: string;
  senderAvatar: string;
  content: string;
  type: MessageType;
  status: MessageStatus;
  createdAt: string; // ISO 8601
  replyTo?: string; // 引用回复的消息 ID
  mentions?: string[]; // @提及的用户 ID 列表
  file?: FileAttachment;
}

export interface SendMessageRequest {
  conversationId: string;
  content: string;
  type: MessageType;
  replyTo?: string;
  mentions?: string[];
  file?: FileAttachment;
}

// ---------- 联系人 ----------
export interface Contact {
  userId: string;
  displayName: string;
  username: string;
  avatar: string;
  status: UserStatus;
  lastSeen?: string;
  isFriend: boolean;
}

// ---------- 好友请求 ----------
export type FriendRequestStatus = "pending" | "accepted" | "rejected";

export interface FriendRequest {
  requestId: string;
  fromUserId: string;
  toUserId: string;
  fromUser?: User;
  toUser?: User;
  message: string;
  status: FriendRequestStatus;
  createdAt: string;
  updatedAt: string;
}

// 前端专用：搜索用户结果（含好友状态）
export interface SearchedUser extends User {
  isFriend: boolean;
  hasSentRequest: boolean;
  hasIncomingRequest: boolean;
}

// ---------- 群组 ----------
export interface CreateGroupRequest {
  name: string;
  avatar?: string;
  memberIds: string[];
}

export interface Group {
  groupId: string;
  name: string;
  avatar: string;
  ownerId: string;
  memberCount: number;
  introduction?: string;
  notification?: string;
  needVerification?: boolean;
  isMuted?: boolean;
  createdAt: string;
}

// 群成员
export interface GroupMemberInfo {
  userId: string;
  groupId: string;
  displayName: string;
  username: string;
  avatar: string;
  roleLevel: number; // 0=普通成员, 1=管理员, 2=群主
  muteEndTime: number; // Unix 时间戳毫秒，0 表示未禁言
  joinedAt: string;
}

// 入群申请
export type GroupApplicationStatus = "pending" | "accepted" | "rejected";

export interface GroupApplication {
  applicationId: string;
  groupId: string;
  userId: string;
  user?: User;
  groupName?: string;
  message: string;
  status: GroupApplicationStatus;
  handleUserId?: string;
  handleMsg?: string;
  createdAt: string;
  updatedAt: string;
}

// 群信息更新
export interface UpdateGroupRequest {
  groupId: string;
  name?: string;
  avatar?: string;
  introduction?: string;
  notification?: string;
  needVerification?: boolean;
}

// ---------- WebSocket 消息协议 ----------
export interface WsMessage<T = unknown> {
  type: WsMessageType;
  payload: T;
  timestamp: string;
}

export type WsMessageType =
  | "message.new" // 新消息
  | "message.read" // 已读回执
  | "message.revoke" // 撤回
  | "conversation.update" // 会话更新
  | "user.status" // 用户在线状态变化
  | "typing" // 正在输入
  | "friend.request" // 好友请求推送（新申请 / 被接受 / 被拒绝）
  | "ping" // 心跳
  | "pong"; // 心跳响应

export interface TypingPayload {
  conversationId: string;
  userId: string;
  isTyping: boolean;
}

// ---------- 好友请求推送 payload ----------
// 对应后端 pkg/notification/tips.go 中的 JSON 结构
export interface FriendRequestPushPayload {
  from_user_id: string;
  to_user_id: string;
  apply_msg?: string;
  apply_time?: number;
  handle_msg?: string;
  handle_time?: number;
}

// ---------- API 响应 ----------
export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
}

export interface PaginatedResponse<T> {
  code: number;
  message: string;
  data: {
    list: T[];
    total: number;
    page: number;
    pageSize: number;
  };
}
