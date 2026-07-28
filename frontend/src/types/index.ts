// ============================================================
// SuIM 即时通讯系统 — 前端类型定义
// ============================================================

// ---------- 用户 ----------
export type UserStatus = "online" | "offline" | "away" | "busy";

export interface User {
  userId: string;
  suid: string;
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
}

export interface SendMessageRequest {
  conversationId: string;
  content: string;
  type: MessageType;
  replyTo?: string;
  mentions?: string[];
}

// ---------- 搜索用户（带好友关系状态）----------
export interface SearchedUser extends User {
  isFriend: boolean;
  hasSentRequest: boolean;
  hasIncomingRequest: boolean;
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
export type FriendRequestStatus = "pending" | "accepted" | "declined";

export interface FriendRequest {
  requestId: string;
  fromUserId: string;
  fromUsername: string;
  fromDisplayName: string;
  fromAvatar: string;
  toUserId: string;
  message: string;
  status: FriendRequestStatus;
  createdAt: string;
  fromUser?: User;
  toUser?: User;
  updatedAt?: string;
}

// ---------- 通知 ----------
export type NotificationType = "friend_request" | "group_invite" | "mention" | "system";

export interface Notification {
  notificationId: string;
  type: NotificationType;
  title: string;
  content: string;
  isRead: boolean;
  refId?: string; // 关联 ID（会话/请求/群组等）
  createdAt: string;
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
  createdAt: string;
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
  | "ping" // 心跳
  | "pong"; // 心跳响应

export interface TypingPayload {
  conversationId: string;
  userId: string;
  isTyping: boolean;
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
