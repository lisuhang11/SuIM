import type { Contact, Conversation, Group, Message, User } from "@/types";

export const isMockMode =
  process.env.NEXT_PUBLIC_MOCK_MODE !== "false" &&
  process.env.NODE_ENV === "development";

export const mockCurrentUser: User = {
  userId: "u_10001",
  username: "lisuhang",
  displayName: "李稣航",
  email: "lisuhang@suim.dev",
  avatar: "https://i.pravatar.cc/160?img=12",
  status: "online",
  createdAt: "2026-01-12T09:20:00+08:00",
};

export const mockContacts: Contact[] = [
  { userId: "u_10002", displayName: "林晚", username: "linwan", avatar: "https://i.pravatar.cc/160?img=47", status: "online", isFriend: true },
  { userId: "u_10003", displayName: "周屿", username: "zhouyu", avatar: "https://i.pravatar.cc/160?img=11", status: "away", isFriend: true },
  { userId: "u_10004", displayName: "陈默", username: "chenmo", avatar: "https://i.pravatar.cc/160?img=15", status: "busy", isFriend: true },
  { userId: "u_10005", displayName: "许棠", username: "xutang", avatar: "https://i.pravatar.cc/160?img=32", status: "offline", lastSeen: "2026-07-29T09:42:00+08:00", isFriend: true },
  { userId: "u_10006", displayName: "赵一鸣", username: "zhaoyiming", avatar: "https://i.pravatar.cc/160?img=5", status: "online", isFriend: true },
  { userId: "u_10007", displayName: "顾清", username: "guqing", avatar: "https://i.pravatar.cc/160?img=44", status: "offline", lastSeen: "2026-07-28T22:18:00+08:00", isFriend: true },
];

const member = (userId: string, conversationId: string, role: "owner" | "admin" | "member" = "member") => ({
  userId,
  conversationId,
  role,
  joinedAt: "2026-03-18T10:00:00+08:00",
});

const lastMessage = (
  messageId: string,
  conversationId: string,
  senderId: string,
  senderName: string,
  content: string,
  createdAt: string,
  status: Message["status"] = "delivered",
): Message => ({
  messageId,
  conversationId,
  senderId,
  senderName,
  senderAvatar: mockContacts.find((item) => item.userId === senderId)?.avatar || "",
  content,
  type: "text",
  status,
  createdAt,
});

export const mockConversations: Conversation[] = [
  {
    conversationId: "single_u_10002",
    type: "private",
    title: "林晚",
    avatar: "https://i.pravatar.cc/160?img=47",
    unreadCount: 2,
    isPinned: true,
    isMuted: false,
    members: [member("u_10001", "single_u_10002"), member("u_10002", "single_u_10002")],
    lastMessage: lastMessage("m_1008", "single_u_10002", "u_10002", "林晚", "交互稿我看完了，消息状态这块很清楚。", "2026-07-29T10:26:00+08:00"),
    createdAt: "2026-05-02T11:00:00+08:00",
    updatedAt: "2026-07-29T10:26:00+08:00",
  },
  {
    conversationId: "group_product",
    type: "group",
    title: "SuIM 产品与研发",
    avatar: "",
    unreadCount: 7,
    isPinned: true,
    isMuted: false,
    members: [member("u_10001", "group_product", "owner"), member("u_10002", "group_product", "admin"), member("u_10003", "group_product"), member("u_10004", "group_product"), member("u_10006", "group_product")],
    lastMessage: lastMessage("m_2010", "group_product", "u_10003", "周屿", "周屿：网关接口已补上会话免打扰字段。", "2026-07-29T10:18:00+08:00"),
    createdAt: "2026-02-08T09:00:00+08:00",
    updatedAt: "2026-07-29T10:18:00+08:00",
  },
  {
    conversationId: "single_u_10003",
    type: "private",
    title: "周屿",
    avatar: "https://i.pravatar.cc/160?img=11",
    unreadCount: 0,
    isPinned: false,
    isMuted: false,
    members: [member("u_10001", "single_u_10003"), member("u_10003", "single_u_10003")],
    lastMessage: lastMessage("m_3004", "single_u_10003", "u_10001", "李稣航", "收到，我今晚把群申请流程串起来。", "2026-07-29T09:41:00+08:00", "read"),
    createdAt: "2026-04-09T14:00:00+08:00",
    updatedAt: "2026-07-29T09:41:00+08:00",
  },
  {
    conversationId: "group_weekend",
    type: "group",
    title: "周末羽毛球",
    avatar: "",
    unreadCount: 0,
    isPinned: false,
    isMuted: true,
    members: [member("u_10001", "group_weekend"), member("u_10004", "group_weekend", "owner"), member("u_10005", "group_weekend"), member("u_10006", "group_weekend")],
    lastMessage: lastMessage("m_4002", "group_weekend", "u_10004", "陈默", "陈默：周六下午两点，还是老场地。", "2026-07-28T21:32:00+08:00"),
    createdAt: "2026-06-11T19:00:00+08:00",
    updatedAt: "2026-07-28T21:32:00+08:00",
  },
  {
    conversationId: "single_u_10005",
    type: "private",
    title: "许棠",
    avatar: "https://i.pravatar.cc/160?img=32",
    unreadCount: 0,
    isPinned: false,
    isMuted: false,
    members: [member("u_10001", "single_u_10005"), member("u_10005", "single_u_10005")],
    lastMessage: lastMessage("m_5002", "single_u_10005", "u_10005", "许棠", "好的，明天同步。", "2026-07-27T18:05:00+08:00"),
    createdAt: "2026-03-21T16:00:00+08:00",
    updatedAt: "2026-07-27T18:05:00+08:00",
  },
];

export const mockMessages: Record<string, Message[]> = {
  single_u_10002: [
    lastMessage("m_1001", "single_u_10002", "u_10002", "林晚", "早上好，昨晚的会话同步问题已经复现了。", "2026-07-29T09:52:00+08:00", "read"),
    lastMessage("m_1002", "single_u_10002", "u_10001", "李稣航", "是增量拉取时序列号没有更新吗？", "2026-07-29T09:55:00+08:00", "read"),
    lastMessage("m_1003", "single_u_10002", "u_10002", "林晚", "对，重连后会重复拿到最后一批消息。", "2026-07-29T09:56:00+08:00", "read"),
    lastMessage("m_1004", "single_u_10002", "u_10001", "李稣航", "我把客户端 seq 和服务端 max_seq 的更新放到同一个确认流程里了。", "2026-07-29T10:07:00+08:00", "read"),
    { ...lastMessage("m_1005", "single_u_10002", "system", "系统", "以下是新消息", "2026-07-29T10:20:00+08:00"), type: "system" },
    lastMessage("m_1006", "single_u_10002", "u_10002", "林晚", "现在正常了。我顺手检查了置顶和免打扰，也都能同步。", "2026-07-29T10:22:00+08:00"),
    lastMessage("m_1007", "single_u_10002", "u_10001", "李稣航", "很好，我再补一下前端的已读反馈。", "2026-07-29T10:24:00+08:00", "delivered"),
    lastMessage("m_1008", "single_u_10002", "u_10002", "林晚", "交互稿我看完了，消息状态这块很清楚。", "2026-07-29T10:26:00+08:00"),
  ],
  group_product: [
    lastMessage("m_2001", "group_product", "u_10004", "陈默", "今天先对齐三个核心流程：会话同步、好友申请、群管理。", "2026-07-29T09:15:00+08:00"),
    lastMessage("m_2002", "group_product", "u_10002", "林晚", "前端会把请求中、失败和空状态都补齐。", "2026-07-29T09:18:00+08:00"),
    lastMessage("m_2003", "group_product", "u_10001", "李稣航", "消息发送继续采用本地回显，服务端确认后替换 server_msg_id。", "2026-07-29T09:23:00+08:00", "read"),
    lastMessage("m_2004", "group_product", "u_10003", "周屿", "网关接口已补上会话免打扰字段。", "2026-07-29T10:18:00+08:00"),
  ],
  single_u_10003: [
    lastMessage("m_3001", "single_u_10003", "u_10003", "周屿", "群申请的 handle_result 还是 1 接受、-1 拒绝。", "2026-07-29T09:34:00+08:00"),
    lastMessage("m_3002", "single_u_10003", "u_10001", "李稣航", "了解，前端状态映射会和好友申请保持一致。", "2026-07-29T09:37:00+08:00", "read"),
    lastMessage("m_3004", "single_u_10003", "u_10001", "李稣航", "收到，我今晚把群申请流程串起来。", "2026-07-29T09:41:00+08:00", "read"),
  ],
};

export const mockGroups: Group[] = [
  { groupId: "group_product", name: "SuIM 产品与研发", avatar: "", ownerId: "u_10001", memberCount: 12, introduction: "围绕 SuIM 的产品、客户端与微服务协作。", notification: "本周目标：完成会话同步与群申请闭环。", needVerification: true, createdAt: "2026-02-08T09:00:00+08:00" },
  { groupId: "group_weekend", name: "周末羽毛球", avatar: "", ownerId: "u_10004", memberCount: 8, introduction: "运动、约场和临时鸽子统计。", notification: "周六 14:00，城北体育馆 3 号场。", needVerification: false, isMuted: true, createdAt: "2026-06-11T19:00:00+08:00" },
  { groupId: "group_backend", name: "后端架构讨论", avatar: "", ownerId: "u_10003", memberCount: 6, introduction: "服务治理、消息可靠性和存储设计。", notification: "接口变更请同步 proto 与网关 handler。", needVerification: true, createdAt: "2026-04-20T14:30:00+08:00" },
];

export const mockFriendRequests = [
  { id: "fr_1", name: "江澄", username: "jiangcheng", avatar: "https://i.pravatar.cc/160?img=13", message: "你好，我在 OpenIM 学习群看到你的项目。", time: "18 分钟前", mutual: 3 },
  { id: "fr_2", name: "沈知夏", username: "shenzhixia", avatar: "https://i.pravatar.cc/160?img=49", message: "想交流一下消息可靠性设计。", time: "昨天", mutual: 1 },
];
