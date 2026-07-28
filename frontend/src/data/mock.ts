// ============================================================
// SuIM Mock 数据 — 开发调试用
// ============================================================
import type { User, Conversation, Message, Contact, Group } from "@/types";

// ---------- 模拟用户 ----------
export const mockUsers: User[] = [
  {
    userId: "u_1001",
    suid: "su_1001",
    username: "zhangsan",
    displayName: "张三",
    avatar: "",
    email: "zhangsan@suim.dev",
    status: "online",
    lastSeen: new Date().toISOString(),
    createdAt: "2025-01-15T08:00:00Z",
  },
  {
    userId: "u_1002",
    suid: "su_1002",
    username: "lisi",
    displayName: "李四",
    avatar: "",
    email: "lisi@suim.dev",
    status: "online",
    lastSeen: new Date().toISOString(),
    createdAt: "2025-02-10T10:30:00Z",
  },
  {
    userId: "u_1003",
    suid: "su_1003",
    username: "wangwu",
    displayName: "王五",
    avatar: "",
    email: "wangwu@suim.dev",
    status: "away",
    lastSeen: new Date(Date.now() - 300000).toISOString(),
    createdAt: "2025-03-05T14:00:00Z",
  },
  {
    userId: "u_1004",
    suid: "su_1004",
    username: "zhaoliu",
    displayName: "赵六",
    avatar: "",
    email: "zhaoliu@suim.dev",
    status: "busy",
    lastSeen: new Date(Date.now() - 60000).toISOString(),
    createdAt: "2025-03-20T09:00:00Z",
  },
  {
    userId: "u_1005",
    suid: "su_1005",
    username: "sunqi",
    displayName: "孙七",
    avatar: "",
    email: "sunqi@suim.dev",
    status: "offline",
    lastSeen: new Date(Date.now() - 86400000).toISOString(),
    createdAt: "2025-04-01T11:00:00Z",
  },
  {
    userId: "u_1006",
    suid: "su_1006",
    username: "zhouba",
    displayName: "周八",
    avatar: "",
    email: "zhouba@suim.dev",
    status: "online",
    lastSeen: new Date().toISOString(),
    createdAt: "2025-04-10T16:00:00Z",
  },
];

// ---------- 当前登录用户 ----------
export const mockCurrentUser: User = mockUsers[0]; // 张三作为当前用户

const now = Date.now();

function ago(ms: number): string {
  return new Date(now - ms).toISOString();
}

// ---------- 模拟消息 ----------
export const mockMessages: Record<string, Message[]> = {
  // 张三 <-> 李四 的私聊
  conv_private_1: [
    {
      messageId: "msg_001", conversationId: "conv_private_1", senderId: "u_1002",
      senderName: "李四", senderAvatar: "", content: "张三，在吗？", type: "text",
      status: "read", createdAt: ago(3600000),
    },
    {
      messageId: "msg_002", conversationId: "conv_private_1", senderId: "u_1001",
      senderName: "张三", senderAvatar: "", content: "在的，什么事？", type: "text",
      status: "read", createdAt: ago(3500000),
    },
    {
      messageId: "msg_003", conversationId: "conv_private_1", senderId: "u_1002",
      senderName: "李四", senderAvatar: "", content: "下午的代码评审你参加吗？", type: "text",
      status: "read", createdAt: ago(3400000),
    },
    {
      messageId: "msg_004", conversationId: "conv_private_1", senderId: "u_1001",
      senderName: "张三", senderAvatar: "", content: "参加的，我准备好 PPT 了 😊", type: "text",
      status: "read", createdAt: ago(3300000),
    },
    {
      messageId: "msg_005", conversationId: "conv_private_1", senderId: "u_1002",
      senderName: "李四", senderAvatar: "", content: "太好了！那我先拉个会", type: "text",
      status: "delivered", createdAt: ago(3000000),
    },
    {
      messageId: "msg_006", conversationId: "conv_private_1", senderId: "u_1002",
      senderName: "李四", senderAvatar: "", content: "对了，新版本 WebSocket 模块你已经改好了吗？", type: "text",
      status: "delivered", createdAt: ago(60000),
    },
  ],
  // 张三 <-> 王五 的私聊
  conv_private_2: [
    {
      messageId: "msg_101", conversationId: "conv_private_2", senderId: "u_1003",
      senderName: "王五", senderAvatar: "", content: "张工，API 网关那个 Bug 我修好了", type: "text",
      status: "read", createdAt: ago(7200000),
    },
    {
      messageId: "msg_102", conversationId: "conv_private_2", senderId: "u_1001",
      senderName: "张三", senderAvatar: "", content: "好的，我 review 一下", type: "text",
      status: "read", createdAt: ago(7100000),
    },
    {
      messageId: "msg_103", conversationId: "conv_private_2", senderId: "u_1003",
      senderName: "王五", senderAvatar: "", content: "主要是 jwt 中间件的 token 刷新逻辑改了", type: "text",
      status: "delivered", createdAt: ago(6900000),
    },
  ],
  // SuIM 开发组 群聊
  conv_group_1: [
    {
      messageId: "msg_201", conversationId: "conv_group_1", senderId: "u_1004",
      senderName: "赵六", senderAvatar: "", content: "大家早上好！今天 Sprint Review，都准备好了吗？", type: "text",
      status: "delivered", createdAt: ago(14400000),
    },
    {
      messageId: "msg_202", conversationId: "conv_group_1", senderId: "u_1002",
      senderName: "李四", senderAvatar: "", content: "准备好了，消息模块的单元测试补充完了", type: "text",
      status: "delivered", createdAt: ago(14300000),
    },
    {
      messageId: "msg_203", conversationId: "conv_group_1", senderId: "u_1006",
      senderName: "周八", senderAvatar: "", content: "群组服务的权限控制我加好了，PR 已经提了", type: "text",
      status: "delivered", createdAt: ago(14200000),
    },
    {
      messageId: "msg_204", conversationId: "conv_group_1", senderId: "u_1001",
      senderName: "张三", senderAvatar: "", content: "👍 大家效率很高", type: "text",
      status: "delivered", createdAt: ago(14100000),
    },
    {
      messageId: "msg_205", conversationId: "conv_group_1", senderId: "u_1003",
      senderName: "王五", senderAvatar: "", content: "对了，@张三 前端的 WebSocket 重连逻辑什么时候加？", type: "text",
      status: "delivered", createdAt: ago(1200000),
      mentions: ["u_1001"],
    },
  ],
  // 运维讨论组 群聊
  conv_group_2: [
    {
      messageId: "msg_301", conversationId: "conv_group_2", senderId: "u_1006",
      senderName: "周八", senderAvatar: "", content: "生产环境 gRPC 连接池我调了一下参数", type: "text",
      status: "delivered", createdAt: ago(86400000),
    },
    {
      messageId: "msg_302", conversationId: "conv_group_2", senderId: "u_1004",
      senderName: "赵六", senderAvatar: "", content: "什么参数？", type: "text",
      status: "delivered", createdAt: ago(86380000),
    },
    {
      messageId: "msg_303", conversationId: "conv_group_2", senderId: "u_1006",
      senderName: "周八", senderAvatar: "", content: "max_idle 从 10 改到 50，keepalive 从 30s 改到 60s", type: "text",
      status: "delivered", createdAt: ago(86360000),
    },
  ],
};

// ---------- 模拟会话 ----------
export const mockConversations: Conversation[] = [
  {
    conversationId: "conv_private_1",
    type: "private",
    title: "李四",
    avatar: "",
    unreadCount: 2,
    isPinned: true,
    isMuted: false,
    members: [
      { userId: "u_1001", conversationId: "conv_private_1", role: "member", joinedAt: "2025-03-01T00:00:00Z" },
      { userId: "u_1002", conversationId: "conv_private_1", role: "member", joinedAt: "2025-03-01T00:00:00Z" },
    ],
    lastMessage: mockMessages.conv_private_1[mockMessages.conv_private_1.length - 1],
    createdAt: "2025-03-01T00:00:00Z",
    updatedAt: ago(60000),
  },
  {
    conversationId: "conv_private_2",
    type: "private",
    title: "王五",
    avatar: "",
    unreadCount: 0,
    isPinned: false,
    isMuted: false,
    members: [
      { userId: "u_1001", conversationId: "conv_private_2", role: "member", joinedAt: "2025-04-01T00:00:00Z" },
      { userId: "u_1003", conversationId: "conv_private_2", role: "member", joinedAt: "2025-04-01T00:00:00Z" },
    ],
    lastMessage: mockMessages.conv_private_2[mockMessages.conv_private_2.length - 1],
    createdAt: "2025-04-01T00:00:00Z",
    updatedAt: ago(6900000),
  },
  {
    conversationId: "conv_group_1",
    type: "group",
    title: "SuIM 开发组",
    avatar: "",
    unreadCount: 5,
    isPinned: false,
    isMuted: false,
    members: [
      { userId: "u_1001", conversationId: "conv_group_1", role: "admin", joinedAt: "2025-05-01T00:00:00Z" },
      { userId: "u_1002", conversationId: "conv_group_1", role: "member", joinedAt: "2025-05-01T00:00:00Z" },
      { userId: "u_1003", conversationId: "conv_group_1", role: "member", joinedAt: "2025-05-01T00:00:00Z" },
      { userId: "u_1004", conversationId: "conv_group_1", role: "owner", joinedAt: "2025-05-01T00:00:00Z" },
      { userId: "u_1006", conversationId: "conv_group_1", role: "member", joinedAt: "2025-05-01T00:00:00Z" },
    ],
    lastMessage: mockMessages.conv_group_1[mockMessages.conv_group_1.length - 1],
    createdAt: "2025-05-01T00:00:00Z",
    updatedAt: ago(1200000),
  },
  {
    conversationId: "conv_group_2",
    type: "group",
    title: "运维讨论组",
    avatar: "",
    unreadCount: 0,
    isPinned: false,
    isMuted: true,
    members: [
      { userId: "u_1001", conversationId: "conv_group_2", role: "member", joinedAt: "2025-06-01T00:00:00Z" },
      { userId: "u_1004", conversationId: "conv_group_2", role: "owner", joinedAt: "2025-06-01T00:00:00Z" },
      { userId: "u_1006", conversationId: "conv_group_2", role: "admin", joinedAt: "2025-06-01T00:00:00Z" },
    ],
    lastMessage: mockMessages.conv_group_2[mockMessages.conv_group_2.length - 1],
    createdAt: "2025-06-01T00:00:00Z",
    updatedAt: ago(86360000),
  },
];

// ---------- 模拟联系人 ----------
export const mockContacts: Contact[] = mockUsers
  .filter((u) => u.userId !== "u_1001")
  .map((u) => ({
    userId: u.userId,
    displayName: u.displayName,
    username: u.username,
    avatar: u.avatar,
    status: u.status,
    lastSeen: u.lastSeen,
    isFriend: true,
  }));

// ---------- 模拟好友请求 ----------
import type { FriendRequest } from "@/types";

export const mockIncomingRequests: FriendRequest[] = [
  {
    requestId: "req_001",
    fromUserId: "u_1005",
    fromUsername: "sunqi",
    fromDisplayName: "孙七",
    fromAvatar: "",
    toUserId: "u_1001",
    fromUser: mockUsers[4], // 孙七
    message: "你好张三，我是孙七，加个好友方便沟通项目进度",
    status: "pending",
    createdAt: new Date(Date.now() - 3600000).toISOString(),
    updatedAt: new Date(Date.now() - 3600000).toISOString(),
  },
  {
    requestId: "req_002",
    fromUserId: "u_1006",
    fromUsername: "zhouba",
    fromDisplayName: "周八",
    fromAvatar: "",
    toUserId: "u_1001",
    fromUser: mockUsers[5], // 周八
    message: "张工好，我是测试组的周八",
    status: "pending",
    createdAt: new Date(Date.now() - 7200000).toISOString(),
    updatedAt: new Date(Date.now() - 7200000).toISOString(),
  },
];

export const mockOutgoingRequests: FriendRequest[] = [
  {
    requestId: "req_003",
    fromUserId: "u_1001",
    fromUsername: "zhangsan",
    fromDisplayName: "张三",
    fromAvatar: "",
    toUserId: "u_1004",
    toUser: mockUsers[3], // 赵六
    message: "赵六你好，想加你好友讨论一下上次的方案",
    status: "pending",
    createdAt: new Date(Date.now() - 86400000).toISOString(),
    updatedAt: new Date(Date.now() - 86400000).toISOString(),
  },
];

export const mockSentRequestUserIds = new Set(
  mockOutgoingRequests.map((r) => r.toUserId)
);
export const mockIncomingRequestUserIds = new Set(
  mockIncomingRequests.map((r) => r.fromUserId)
);

// ---------- 模拟群组 ----------
export const mockGroups: Group[] = [
  {
    groupId: "g_1001", name: "SuIM 开发组", avatar: "", ownerId: "u_1004",
    memberCount: 5, createdAt: "2025-05-01T00:00:00Z",
  },
  {
    groupId: "g_1002", name: "运维讨论组", avatar: "", ownerId: "u_1004",
    memberCount: 3, createdAt: "2025-06-01T00:00:00Z",
  },
];

// ---------- 辅助函数 ----------
export function getUserById(userId: string): User | undefined {
  return mockUsers.find((u) => u.userId === userId);
}

export function getConversationById(id: string): Conversation | undefined {
  return mockConversations.find((c) => c.conversationId === id);
}

export function getMessagesByConversationId(id: string): Message[] {
  return mockMessages[id] || [];
}

export function getStatusColor(status: User["status"]): string {
  switch (status) {
    case "online":  return "#22c55e";
    case "away":    return "#f59e0b";
    case "busy":    return "#ef4444";
    case "offline": return "#9ca3af";
  }
}

export function getStatusText(status: User["status"]): string {
  switch (status) {
    case "online":  return "在线";
    case "away":    return "离开";
    case "busy":    return "忙碌";
    case "offline": return "离线";
  }
}
