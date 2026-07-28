"use client";

// ============================================================
// ChatContext — 聊天状态全局管理
// ============================================================
import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useRef,
} from "react";
import type {
  Conversation,
  Message,
  SendMessageRequest,
  User,
  Contact,
  Group,
  WsMessage,
} from "@/types";
import { useAuth } from "./AuthContext";
import { wsManager } from "@/services/websocket";
import * as storage from "@/services/storage";
import { toMessage } from "@/services/api";
import {
  isMockMode,
  mockContacts,
  mockConversations,
  mockGroups,
  mockMessages,
} from "@/services/mock-data";

interface ChatState {
  conversations: Conversation[];
  activeConversationId: string | null;
  messages: Record<string, Message[]>; // conversationId -> messages
  contacts: Contact[];
  groups: Group[];
  typingUsers: Record<string, string[]>; // conversationId -> userId[]
  wsConnected: boolean;
  isLoading: boolean;
  friendRequestBadge: number;
  friendRequestVersion: number;
}

interface ChatContextValue extends ChatState {
  setActiveConversation: (id: string) => void;
  sendMessage: (req: SendMessageRequest) => Promise<Message>;
  sendTyping: (conversationId: string, isTyping: boolean) => void;
  markConversationRead: (conversationId: string) => void;
  addConversation: (conv: Conversation) => void;
  removeConversation: (id: string) => void;
  refreshConversations: () => Promise<void>;
  refreshGroups: () => Promise<void>;
  loadMessages: (conversationId: string) => Promise<void>;
  searchContacts: (query: string) => Contact[];
  createGroup: (name: string, memberIds: string[]) => Promise<Conversation | null>;
  refreshFriendRequestBadge: () => Promise<void>;
}

const ChatContext = createContext<ChatContextValue | null>(null);

export function ChatProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();

  const [state, setState] = useState<ChatState>({
    conversations: [],
    activeConversationId:
      storage.getActiveConversationId() || null,
    messages: {},
    contacts: [],
    groups: [],
    typingUsers: {},
    wsConnected: false,
    isLoading: true,
    friendRequestBadge: 0,
    friendRequestVersion: 0,
  });

  const stateRef = useRef(state);
  stateRef.current = state;

  // ---------- 初始化：从后端 API 加载数据 ----------
  useEffect(() => {
    if (!user) return;

    const loadData = async () => {
      setState((s) => ({ ...s, isLoading: true }));
      if (isMockMode) {
        setState((s) => ({
          ...s,
          conversations: mockConversations,
          messages: mockMessages,
          contacts: mockContacts,
          groups: mockGroups,
          activeConversationId: s.activeConversationId || mockConversations[0].conversationId,
          wsConnected: true,
          friendRequestBadge: 2,
          isLoading: false,
        }));
        return;
      }
      try {
        // 尝试从真实 API 加载数据
        const api = await import("@/services/api");
        const [convRes, contactRes, groupRes] = await Promise.allSettled([
          api.getConversations(),
          api.getContacts(),
          api.getGroups(),
        ]);

        setState((s) => ({
          ...s,
          conversations: convRes.status === "fulfilled" ? convRes.value : [],
          messages: {},
          contacts: contactRes.status === "fulfilled" ? contactRes.value : [],
          groups: groupRes.status === "fulfilled" ? groupRes.value : [],
          isLoading: false,
        }));
      } catch {
        // API 不可用，使用空数据
        setState((s) => ({
          ...s,
          conversations: [],
          messages: {},
          contacts: [],
          groups: [],
          isLoading: false,
        }));
      }
    };

    loadData();
  }, [user]);

  // ---------- WebSocket 消息监听 ----------
  useEffect(() => {
    const unsubs: (() => void)[] = [];

    if (isMockMode) return;

    if (user) {
      // 监听新消息
      unsubs.push(
        wsManager.on("message.new", (msg: WsMessage) => {
          handleNewMessage(msg);
        })
      );

      // 监听已读回执
      unsubs.push(
        wsManager.on("message.read", (msg: WsMessage) => {
          handleReadReceipt(msg);
        })
      );

      // 监听用户状态
      unsubs.push(
        wsManager.on("user.status", (msg: WsMessage) => {
          handleUserStatus(msg);
        })
      );

      // 监听正在输入
      unsubs.push(
        wsManager.on("typing", (msg: WsMessage) => {
          handleTyping(msg);
        })
      );

      // 监听好友请求推送
      unsubs.push(
        wsManager.on("friend.request", () => {
          refreshFriendRequestBadge();
        })
      );

      // 连接状态
      unsubs.push(
        wsManager.onStatusChange((connected) => {
          setState((s) => ({ ...s, wsConnected: connected }));
        })
      );
    }

    return () => unsubs.forEach((fn) => fn());
  }, [user]);

  // ---------- 处理新消息 ----------
  const handleNewMessage = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as { message: Message & Record<string, unknown> };
    const rawMessage = payload.message;
    const newMsg = rawMessage && ("content_type" in rawMessage || "server_msg_id" in rawMessage)
      ? toMessage(rawMessage)
      : rawMessage;
    if (!newMsg) return;

    // 系统消息（content_type 1000~1099 好友通知）自动刷新好友请求 badge
    if (newMsg.type === "system") {
      refreshFriendRequestBadge();
    }

    setState((prev) => {
      // 添加消息
      const convMessages = prev.messages[newMsg.conversationId] || [];
      const exists = convMessages.some((m) => m.messageId === newMsg.messageId);
      if (exists) return prev;

      const updatedMessages = {
        ...prev.messages,
        [newMsg.conversationId]: [...convMessages, newMsg],
      };

      // 更新最后一条消息
      const updatedConversations = prev.conversations.map((c) => {
        if (c.conversationId === newMsg.conversationId) {
          const unread =
            newMsg.conversationId === prev.activeConversationId
              ? c.unreadCount
              : c.unreadCount + 1;
          return { ...c, lastMessage: newMsg, unreadCount: unread, updatedAt: newMsg.createdAt };
        }
        return c;
      });

      return {
        ...prev,
        messages: updatedMessages,
        conversations: updatedConversations,
      };
    });
  }, []);

  // ---------- 处理已读回执 ----------
  const handleReadReceipt = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as { conversationId: string; userId: string };
    setState((prev) => {
      const convMessages = prev.messages[payload.conversationId];
      if (!convMessages) return prev;
      return {
        ...prev,
        messages: {
          ...prev.messages,
          [payload.conversationId]: convMessages.map((m) =>
            m.senderId === payload.userId ? { ...m, status: "read" as const } : m
          ),
        },
      };
    });
  }, []);

  // ---------- 处理用户状态变更 ----------
  const handleUserStatus = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as { userId: string; status: User["status"] };
    setState((prev) => ({
      ...prev,
      contacts: prev.contacts.map((c) =>
        c.userId === payload.userId ? { ...c, status: payload.status } : c
      ),
    }));
  }, []);

  // ---------- 处理正在输入 ----------
  const handleTyping = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as { conversationId: string; userId: string; isTyping: boolean };
    setState((prev) => {
      const current = prev.typingUsers[payload.conversationId] || [];
      let updated: string[];
      if (payload.isTyping) {
        updated = current.includes(payload.userId) ? current : [...current, payload.userId];
      } else {
        updated = current.filter((id) => id !== payload.userId);
      }
      return {
        ...prev,
        typingUsers: { ...prev.typingUsers, [payload.conversationId]: updated },
      };
    });
  }, []);

  // ---------- 切换活跃会话 ----------
  const setActiveConversation = useCallback((id: string) => {
    storage.setActiveConversationId(id);
    setState((prev) => ({
      ...prev,
      activeConversationId: id,
      conversations: prev.conversations.map((c) =>
        c.conversationId === id ? { ...c, unreadCount: 0 } : c
      ),
    }));
    // 标记已读
    markConversationRead(id);
  }, []);

  // ---------- 发送消息 ----------
  const sendMessage = useCallback(
    async (req: SendMessageRequest): Promise<Message> => {
      // 构造一个本地消息
      const localMsg: Message = {
        messageId: `msg_local_${Date.now()}`,
        conversationId: req.conversationId,
        senderId: user?.userId || "",
        senderName: user?.displayName || user?.username || "",
        senderAvatar: user?.avatar || "",
        content: req.content,
        type: req.type,
        status: "sending",
        createdAt: new Date().toISOString(),
        replyTo: req.replyTo,
        mentions: req.mentions,
        file: req.file,
      };

      // 乐观更新：立即添加到消息列表
      setState((prev) => ({
        ...prev,
        messages: {
          ...prev.messages,
          [req.conversationId]: [
            ...(prev.messages[req.conversationId] || []),
            localMsg,
          ],
        },
        conversations: prev.conversations.map((c) =>
          c.conversationId === req.conversationId
            ? { ...c, lastMessage: localMsg, updatedAt: localMsg.createdAt }
            : c
        ),
      }));

      // 尝试通过 HTTP REST API 发送消息
      if (isMockMode) {
        await new Promise((resolve) => setTimeout(resolve, 280));
        setState((prev) => ({
          ...prev,
          messages: {
            ...prev.messages,
            [req.conversationId]: prev.messages[req.conversationId].map((m) =>
              m.messageId === localMsg.messageId ? { ...m, status: "sent" as const } : m
            ),
          },
        }));
        return { ...localMsg, status: "sent" };
      }

      try {
        const conversation = state.conversations.find((item) => item.conversationId === req.conversationId);
        const content = req.file ? JSON.stringify({
          file_id: req.file.fileId,
          name: req.file.name,
          content_type: req.file.contentType,
          size: req.file.size,
          sha256: req.file.sha256,
          category: req.file.category,
          expires_at: req.file.expiresAt,
        }) : req.content;
        const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL || "/api/v1"}/messages`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "Authorization": `Bearer ${localStorage.getItem("suim_token") || ""}`,
          },
          body: JSON.stringify({
            msg_data: {
              client_msg_id: localMsg.messageId,
              conversation_id: req.conversationId,
              session_type: conversation?.type === "group" ? 2 : 1,
              group_id: conversation?.type === "group" ? req.conversationId : "",
              recv_id: conversation?.type === "private" ? conversation.members.find((member) => member.userId !== user?.userId)?.userId || "" : "",
              recv_user_ids: conversation?.members.filter((member) => member.userId !== user?.userId).map((member) => member.userId) || [],
              content_type: req.type === "image" ? 1 : req.type === "file" ? 2 : 0,
              content,
              sender_nickname: user?.displayName || user?.username || "",
              sender_face_url: user?.avatar || "",
            },
          }),
        });
        if (!response.ok) {
          const body = await response.json().catch(() => ({ message: response.statusText }));
          throw new Error(body.message || "消息发送失败");
        }
      } catch (error) {
        if (req.file) {
          setState((prev) => ({
            ...prev,
            messages: { ...prev.messages, [req.conversationId]: prev.messages[req.conversationId].map((message) => message.messageId === localMsg.messageId ? { ...message, status: "failed" } : message) },
          }));
          throw error;
        }
        // HTTP API 不可用，尝试通过 WebSocket 发送
        wsManager.send("message.new" as never, { message: localMsg });
      }

      // 更新消息状态
      const finalStatus = "sent";
      setState((prev) => ({
        ...prev,
        messages: {
          ...prev.messages,
          [req.conversationId]: prev.messages[req.conversationId].map((m) =>
            m.messageId === localMsg.messageId ? { ...m, status: finalStatus } : m
          ),
        },
      }));

      return { ...localMsg, status: finalStatus };
    },
    [state.conversations, user]
  );

  // ---------- 发送正在输入状态 ----------
  const sendTyping = useCallback(
    (conversationId: string, isTyping: boolean) => {
      wsManager.send("typing", {
        conversationId,
        userId: user?.userId,
        isTyping,
      });
    },
    [user]
  );

  // ---------- 标记已读 ----------
  const markConversationRead = useCallback((conversationId: string) => {
    wsManager.send("message.read" as any, { conversationId });
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.map((c) =>
        c.conversationId === conversationId ? { ...c, unreadCount: 0 } : c
      ),
    }));
  }, []);

  // ---------- 添加会话 ----------
  const addConversation = useCallback((conv: Conversation) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.some((item) => item.conversationId === conv.conversationId)
        ? prev.conversations.map((item) => item.conversationId === conv.conversationId ? conv : item)
        : [conv, ...prev.conversations],
    }));
  }, []);

  // ---------- 删除会话 ----------
  const removeConversation = useCallback((id: string) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.filter((c) => c.conversationId !== id),
      activeConversationId:
        prev.activeConversationId === id ? null : prev.activeConversationId,
    }));
  }, []);

  // ---------- 刷新会话列表 ----------
  const refreshConversations = useCallback(async () => {
    try {
      const api = await import("@/services/api");
      const convs = await api.getConversations();
      if (convs.length > 0) {
        setState((prev) => ({
          ...prev,
          conversations: convs,
        }));
      }
    } catch {
      // API 不可用，保持当前数据
    }
  }, []);

  // ---------- 加载消息 ----------
  const loadMessages = useCallback(async (conversationId: string) => {
    if (isMockMode) return;
    try {
      const { getMessages } = await import("@/services/api");
      const msgs = await getMessages(conversationId, { limit: 50 });
      setState((prev) => ({
        ...prev,
        messages: { ...prev.messages, [conversationId]: msgs },
      }));
    } catch {
      // API 不可用，保持当前数据
    }
  }, []);

  // ---------- 搜索联系人 ----------
  const searchContacts = useCallback(
    (query: string): Contact[] => {
      if (!query.trim()) return state.contacts;
      const q = query.toLowerCase();
      return state.contacts.filter(
        (c) =>
          c.displayName.toLowerCase().includes(q) ||
          c.username.toLowerCase().includes(q)
      );
    },
    [state.contacts]
  );

  // ---------- 创建群组 ----------
  const createGroup = useCallback(
    async (
      name: string,
      memberIds: string[]
    ): Promise<Conversation | null> => {
      // 尝试通过真实 API 创建群组
      try {
        if (isMockMode) throw new Error("mock mode");
        const api = await import("@/services/api");
        const conv = await api.createGroupConversation({ name, memberIds });
        setState((prev) => ({
          ...prev,
          conversations: [conv, ...prev.conversations],
          groups: [
            ...prev.groups,
            {
              groupId: conv.conversationId,
              name,
              avatar: conv.avatar || "",
              ownerId: user?.userId || "",
              memberCount: memberIds.length + 1,
              createdAt: conv.createdAt,
            },
          ],
        }));
        return conv;
      } catch {
        // 回退：本地创建
        const newConv: Conversation = {
          conversationId: `conv_group_${Date.now()}`,
          type: "group",
          title: name,
          avatar: "",
          unreadCount: 0,
          isPinned: false,
          isMuted: false,
          members: [
            {
              userId: user?.userId || "",
              conversationId: "",
              role: "owner",
              joinedAt: new Date().toISOString(),
            },
            ...memberIds.map((id) => ({
              userId: id,
              conversationId: "",
              role: "member" as const,
              joinedAt: new Date().toISOString(),
            })),
          ],
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        };

        setState((prev) => ({
          ...prev,
          conversations: [newConv, ...prev.conversations],
          groups: [
            ...prev.groups,
            {
              groupId: newConv.conversationId,
              name,
              avatar: "",
              ownerId: user?.userId || "",
              memberCount: memberIds.length + 1,
              createdAt: newConv.createdAt,
            },
          ],
        }));

        return newConv;
      }
    },
    [user]
  );

  // ---------- 刷新群组列表 ----------
  const refreshGroups = useCallback(async () => {
    if (isMockMode) return;
    try {
      const api = await import("@/services/api");
      const groups = await api.getGroups();
      if (groups.length > 0) {
        setState((prev) => ({ ...prev, groups }));
      }
    } catch {
      // API 不可用，保持当前数据
    }
  }, []);

  // ---------- 刷新好友请求 badge（WS 推送后调用）----------
  const refreshFriendRequestBadge = useCallback(async () => {
    if (isMockMode) {
      setState((prev) => ({ ...prev, friendRequestBadge: 2 }));
      return;
    }
    try {
      const api = await import("@/services/api");
      const count = await api.getUnhandledRequestCount();
      setState((prev) => ({
        ...prev,
        friendRequestBadge: count,
        friendRequestVersion: prev.friendRequestVersion + 1,
      }));
    } catch {
      // ignore
    }
  }, []);

  return (
    <ChatContext.Provider
      value={{
        ...state,
        setActiveConversation,
        sendMessage,
        sendTyping,
        markConversationRead,
        addConversation,
        removeConversation,
        refreshConversations,
        refreshGroups,
        loadMessages,
        searchContacts,
        createGroup,
        refreshFriendRequestBadge,
      }}
    >
      {children}
    </ChatContext.Provider>
  );
}

export function useChat(): ChatContextValue {
  const ctx = useContext(ChatContext);
  if (!ctx) throw new Error("useChat must be used within ChatProvider");
  return ctx;
}
