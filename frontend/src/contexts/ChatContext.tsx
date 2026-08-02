"use client";

// ChatContext — chat state (IMSDK)
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
  CallTipsPayload,
} from "@/types";
import { useAuth } from "./AuthContext";
import { IMSDK, toMessage, getIdb } from "@/suim-sdk";
import * as storage from "@/services/storage";
import { toEpochMs } from "@/lib/utils";
import {
  isMockMode,
  mockContacts,
  mockConversations,
  mockGroups,
  mockMessages,
} from "@/services/mock-data";
import CallOverlay from "@/components/chat/CallOverlay";

export type CallUiPhase = "idle" | "incoming" | "outgoing" | "active";

export interface CallSession {
  callId: string;
  conversationId?: string;
  peerUserId: string;
  peerName: string;
  peerAvatar: string;
  mediaType: "audio" | "video";
  role: "caller" | "callee";
  livekitUrl: string;
  token: string;
  muted: boolean;
  activeSince?: number;
}

const RING_TIMEOUT_MS = 45_000;

interface ChatState {
  conversations: Conversation[];
  activeConversationId: string | null;
  messages: Record<string, Message[]>;
  contacts: Contact[];
  groups: Group[];
  typingUsers: Record<string, string[]>;
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
  revokeMessage: (conversationId: string, clientMsgId: string) => Promise<void>;
  addConversation: (conv: Conversation) => void;
  removeConversation: (id: string) => void;
  updateConversation: (id: string, patch: Partial<Conversation>) => void;
  refreshConversations: () => Promise<void>;
  refreshGroups: () => Promise<void>;
  loadMessages: (conversationId: string) => Promise<void>;
  searchContacts: (query: string) => Contact[];
  createGroup: (name: string, memberIds: string[]) => Promise<Conversation | null>;
  refreshFriendRequestBadge: () => Promise<void>;
  refreshContacts: () => Promise<void>;
  openOrCreatePrivateChat: (peerUserId: string) => Promise<string | null>;
  callPhase: CallUiPhase;
  callSession: CallSession | null;
  callBusy: boolean;
  callError: string | null;
  startVoiceCall: (peerUserId: string, peerName: string, peerAvatar?: string) => Promise<void>;
  acceptCall: () => Promise<void>;
  rejectCall: () => Promise<void>;
  cancelCall: () => Promise<void>;
  hangupCall: () => Promise<void>;
  toggleCallMute: () => Promise<void>;
  dismissCallError: () => void;
}

const ChatContext = createContext<ChatContextValue | null>(null);

export function ChatProvider({ children }: { children: React.ReactNode }) {
  const { user } = useAuth();

  const [state, setState] = useState<ChatState>({
    conversations: [],
    activeConversationId: storage.getActiveConversationId() || null,
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

  const [callPhase, setCallPhase] = useState<CallUiPhase>("idle");
  const [callSession, setCallSession] = useState<CallSession | null>(null);
  const [callBusy, setCallBusy] = useState(false);
  const [callError, setCallError] = useState<string | null>(null);
  const callSessionRef = useRef<CallSession | null>(null);
  const callPhaseRef = useRef<CallUiPhase>("idle");
  const ringTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  callSessionRef.current = callSession;
  callPhaseRef.current = callPhase;

  const clearRingTimer = useCallback(() => {
    if (ringTimerRef.current) {
      clearTimeout(ringTimerRef.current);
      ringTimerRef.current = null;
    }
  }, []);

  const resetCall = useCallback(async () => {
    clearRingTimer();
    try {
      await IMSDK.disconnectCallMedia();
    } catch {
      // ignore
    }
    setCallPhase("idle");
    setCallSession(null);
    setCallBusy(false);
  }, [clearRingTimer]);

  const dismissCallError = useCallback(() => setCallError(null), []);

  const resolvePeerProfile = useCallback(
    (peerUserId: string) => {
      const contact = stateRef.current.contacts.find((c) => c.userId === peerUserId);
      return {
        peerName: contact?.displayName || contact?.nickname || peerUserId,
        peerAvatar: contact?.avatar || "",
      };
    },
    []
  );

  const connectActiveCall = useCallback(async (session: CallSession) => {
    if (!session.token || !session.livekitUrl) {
      throw new Error("缺少通话媒体凭证");
    }
    await IMSDK.connectCallMedia({ url: session.livekitUrl, token: session.token });
    const activeSince = Date.now();
    setCallSession((prev) => (prev ? { ...prev, activeSince, muted: false } : prev));
    setCallPhase("active");
  }, []);

  const startVoiceCall = useCallback(
    async (peerUserId: string, peerName: string, peerAvatar = "") => {
      if (isMockMode) {
        setCallError("演示模式不支持语音通话");
        return;
      }
      if (callPhaseRef.current !== "idle") return;
      setCallError(null);
      setCallBusy(true);
      try {
        const res = await IMSDK.inviteCall(peerUserId, "audio");
        const session: CallSession = {
          callId: res.call.callId,
          conversationId: res.call.conversationId,
          peerUserId,
          peerName: peerName || peerUserId,
          peerAvatar,
          mediaType: "audio",
          role: "caller",
          livekitUrl: res.livekitUrl,
          token: res.token,
          muted: false,
        };
        setCallSession(session);
        setCallPhase("outgoing");
        clearRingTimer();
        ringTimerRef.current = setTimeout(() => {
          void (async () => {
            const current = callSessionRef.current;
            if (callPhaseRef.current !== "outgoing" || !current) return;
            try {
              await IMSDK.cancelCall(current.callId);
            } catch {
              // server may already timeout
            }
            await resetCall();
          })();
        }, RING_TIMEOUT_MS);
      } catch (err) {
        setCallError(err instanceof Error ? err.message : "发起通话失败");
        await resetCall();
      } finally {
        setCallBusy(false);
      }
    },
    [clearRingTimer, resetCall]
  );

  const acceptCall = useCallback(async () => {
    const session = callSessionRef.current;
    if (!session || callPhaseRef.current !== "incoming") return;
    setCallBusy(true);
    setCallError(null);
    clearRingTimer();
    try {
      const res = await IMSDK.acceptCall(session.callId);
      const next: CallSession = {
        ...session,
        token: res.token,
        livekitUrl: res.livekitUrl || session.livekitUrl,
        conversationId: res.call.conversationId || session.conversationId,
      };
      setCallSession(next);
      await connectActiveCall(next);
    } catch (err) {
      setCallError(err instanceof Error ? err.message : "接听失败");
      await resetCall();
    } finally {
      setCallBusy(false);
    }
  }, [clearRingTimer, connectActiveCall, resetCall]);

  const rejectCall = useCallback(async () => {
    const session = callSessionRef.current;
    if (!session || callPhaseRef.current !== "incoming") return;
    setCallBusy(true);
    clearRingTimer();
    try {
      await IMSDK.rejectCall(session.callId);
    } catch {
      // ignore
    } finally {
      await resetCall();
      setCallBusy(false);
    }
  }, [clearRingTimer, resetCall]);

  const cancelCall = useCallback(async () => {
    const session = callSessionRef.current;
    if (!session || callPhaseRef.current !== "outgoing") return;
    setCallBusy(true);
    clearRingTimer();
    try {
      await IMSDK.cancelCall(session.callId);
    } catch {
      // ignore
    } finally {
      await resetCall();
      setCallBusy(false);
    }
  }, [clearRingTimer, resetCall]);

  const hangupCall = useCallback(async () => {
    const session = callSessionRef.current;
    if (!session || callPhaseRef.current !== "active") return;
    setCallBusy(true);
    clearRingTimer();
    try {
      await IMSDK.hangupCall(session.callId);
    } catch {
      // ignore
    } finally {
      await resetCall();
      setCallBusy(false);
    }
  }, [clearRingTimer, resetCall]);

  const toggleCallMute = useCallback(async () => {
    const session = callSessionRef.current;
    if (!session || callPhaseRef.current !== "active") return;
    const nextMuted = !session.muted;
    try {
      await IMSDK.setCallMicEnabled(!nextMuted);
      setCallSession({ ...session, muted: nextMuted });
    } catch (err) {
      setCallError(err instanceof Error ? err.message : "切换静音失败");
    }
  }, []);

  const handleCallInvite = useCallback(
    (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as CallTipsPayload;
      if (!payload?.callId || payload.callerId === user?.userId) return;
      if (callPhaseRef.current !== "idle") return;

      const { peerName, peerAvatar } = resolvePeerProfile(payload.callerId || "");
      setCallSession({
        callId: payload.callId,
        conversationId: payload.conversationId,
        peerUserId: payload.callerId || "",
        peerName,
        peerAvatar,
        mediaType: payload.mediaType === "video" ? "video" : "audio",
        role: "callee",
        livekitUrl: "",
        token: "",
        muted: false,
      });
      setCallPhase("incoming");
      clearRingTimer();
      ringTimerRef.current = setTimeout(() => {
        void resetCall();
      }, RING_TIMEOUT_MS);
    },
    [user?.userId, resolvePeerProfile, clearRingTimer, resetCall]
  );

  const handleCallAccepted = useCallback(
    async (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as CallTipsPayload;
      const session = callSessionRef.current;
      if (!payload?.callId || !session || session.callId !== payload.callId) return;
      if (session.role !== "caller" || callPhaseRef.current !== "outgoing") return;
      clearRingTimer();
      setCallBusy(true);
      try {
        await connectActiveCall(session);
      } catch (err) {
        setCallError(err instanceof Error ? err.message : "连接通话失败");
        await resetCall();
      } finally {
        setCallBusy(false);
      }
    },
    [clearRingTimer, connectActiveCall, resetCall]
  );

  const handleCallEndedLike = useCallback(
    async (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as CallTipsPayload;
      const session = callSessionRef.current;
      if (!payload?.callId || !session || session.callId !== payload.callId) return;
      await resetCall();
    },
    [resetCall]
  );

  useEffect(
    () => () => {
      clearRingTimer();
      void IMSDK.disconnectCallMedia();
    },
    [clearRingTimer]
  );

  const updateConversation = useCallback((id: string, patch: Partial<Conversation>) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.map((item) =>
        item.conversationId === id ? { ...item, ...patch } : item
      ),
    }));
  }, []);

  const refreshConversations = useCallback(async () => {
    try {
      const convs = await IMSDK.getAllConversationList();
      setState((prev) => ({
        ...prev,
        conversations: enrichConversations(convs, prev.contacts, user?.userId, prev.groups),
      }));
    } catch {
      // keep current
    }
  }, [user?.userId]);

  const refreshContacts = useCallback(async () => {
    try {
      const contacts = await IMSDK.incrSyncFriends();
      setState((prev) => ({
        ...prev,
        contacts,
        conversations: enrichConversations(prev.conversations, contacts, user?.userId, prev.groups),
      }));
    } catch (err) {
      if (process.env.NODE_ENV === "development") {
        console.warn("[ChatContext] refreshContacts failed:", err);
      }
    }
  }, [user?.userId]);

  const applySyncedFriends = useCallback(
    (contacts: Contact[]) => {
      setState((prev) => ({
        ...prev,
        contacts,
        conversations: enrichConversations(prev.conversations, contacts, user?.userId, prev.groups),
      }));
    },
    [user?.userId]
  );

  const refreshGroups = useCallback(async () => {
    if (isMockMode) return;
    try {
      const groups = await IMSDK.incrSyncJoinedGroups();
      setState((prev) => ({
        ...prev,
        groups,
        conversations: enrichConversations(
          prev.conversations,
          prev.contacts,
          user?.userId,
          groups
        ),
      }));
    } catch {
      // keep current
    }
  }, [user?.userId]);

  const applySyncedGroups = useCallback(
    (groups: Group[]) => {
      setState((prev) => ({
        ...prev,
        groups,
        conversations: enrichConversations(
          prev.conversations,
          prev.contacts,
          user?.userId,
          groups
        ),
      }));
    },
    [user?.userId]
  );

  const refreshFriendRequestBadge = useCallback(async () => {
    if (isMockMode) {
      setState((prev) => ({ ...prev, friendRequestBadge: 2 }));
      return;
    }
    try {
      const count = await IMSDK.getFriendApplicationUnhandledCount();
      setState((prev) => ({
        ...prev,
        friendRequestBadge: count,
        friendRequestVersion: prev.friendRequestVersion + 1,
      }));
    } catch {
      // ignore
    }
  }, []);

  useEffect(() => {
    if (!user) return;
    let cancelled = false;

    const loadData = async () => {
      setState((s) => ({ ...s, isLoading: true }));
      if (isMockMode) {
        if (cancelled) return;
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
        const [convRes, contactRes, groupRes, badgeRes] = await Promise.allSettled([
          IMSDK.getAllConversationList(),
          IMSDK.getFriendList(),
          IMSDK.getJoinedGroupList(),
          IMSDK.getFriendApplicationUnhandledCount(),
        ]);
        if (cancelled) return;

        if (process.env.NODE_ENV === "development") {
          for (const [name, res] of [
            ["conversations", convRes],
            ["friends", contactRes],
            ["groups", groupRes],
            ["friendBadge", badgeRes],
          ] as const) {
            if (res.status === "rejected") {
              console.warn(`[ChatContext] load ${name} failed:`, res.reason);
            }
          }
        }

        const contacts = contactRes.status === "fulfilled" ? contactRes.value : [];
        const groups = groupRes.status === "fulfilled" ? groupRes.value : [];
        setState((s) => ({
          ...s,
          conversations: enrichConversations(
            convRes.status === "fulfilled" ? convRes.value : [],
            contacts,
            user.userId,
            groups
          ),
          messages: {},
          contacts,
          groups,
          friendRequestBadge: badgeRes.status === "fulfilled" ? badgeRes.value : 0,
          isLoading: false,
        }));
      } catch {
        if (cancelled) return;
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

    void loadData();
    return () => {
      cancelled = true;
    };
  }, [user]);

  const handleNewMessage = useCallback(
    (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as { message: Message & Record<string, unknown> };
      const rawMessage = payload?.message;
      if (!rawMessage) return;
      const newMsg =
        "content_type" in rawMessage ||
        "server_msg_id" in rawMessage ||
        "contentType" in rawMessage
          ? toMessage(rawMessage)
          : (rawMessage as Message);
      if (!newMsg?.conversationId) return;

      if (newMsg.type === "system") {
        void refreshFriendRequestBadge();
      }

      let missingConversation = false;
      setState((prev) => {
        const convMessages = prev.messages[newMsg.conversationId] || [];
        const existingIdx = convMessages.findIndex((m) => sameMessage(m, newMsg));
        if (existingIdx >= 0) {
          const merged = [...convMessages];
          const prevMsg = merged[existingIdx];
          merged[existingIdx] = {
            ...newMsg,
            clientMsgId: newMsg.clientMsgId || prevMsg.clientMsgId,
            status: prevMsg.status === "sending" ? "sent" : newMsg.status,
          };
          return {
            ...prev,
            messages: { ...prev.messages, [newMsg.conversationId]: merged },
            conversations: prev.conversations.map((c) =>
              c.conversationId === newMsg.conversationId
                ? {
                    ...c,
                    lastMessage: merged[existingIdx],
                    updatedAt: merged[existingIdx].createdAt,
                  }
                : c
            ),
          };
        }

        missingConversation = !prev.conversations.some(
          (c) => c.conversationId === newMsg.conversationId
        );
        const isSelf = newMsg.senderId && newMsg.senderId === user?.userId;
        const updatedMessages = {
          ...prev.messages,
          [newMsg.conversationId]: [...convMessages, newMsg],
        };
        const updatedConversations = missingConversation
          ? prev.conversations
          : prev.conversations.map((c) => {
              if (c.conversationId !== newMsg.conversationId) return c;
              const unread =
                isSelf || newMsg.conversationId === prev.activeConversationId
                  ? c.unreadCount
                  : c.unreadCount + 1;
              return {
                ...c,
                lastMessage: newMsg,
                unreadCount: unread,
                updatedAt: newMsg.createdAt,
              };
            });

        return {
          ...prev,
          messages: updatedMessages,
          conversations: updatedConversations,
        };
      });

      if (missingConversation) {
        void refreshConversations();
      }

      if (user?.userId && newMsg?.clientMsgId) {
        void getIdb(user.userId).upsertMessages([newMsg]);
      }
    },
    [user?.userId, refreshConversations, refreshFriendRequestBadge]
  );

  const handleReadReceipt = useCallback(
    (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as {
        conversationId: string;
        userId: string;
        hasReadSeq?: number;
        seqs?: number[];
      };
      const myId = user?.userId;
      if (!payload?.conversationId || !myId) return;
      // 对端已读：只更新「我发出的」消息；自己多端 tip 不改气泡状态
      if (payload.userId === myId) return;
      const hasReadSeq = Number(payload.hasReadSeq ?? 0);
      setState((prev) => {
        const convMessages = prev.messages[payload.conversationId];
        if (!convMessages) return prev;
        return {
          ...prev,
          messages: {
            ...prev.messages,
            [payload.conversationId]: convMessages.map((m) => {
              if (m.senderId !== myId || m.status === "revoked") return m;
              const seq = m.seq || 0;
              if (hasReadSeq > 0 && seq > 0 && seq <= hasReadSeq) {
                return { ...m, status: "read" as const };
              }
              if (payload.seqs?.length && seq > 0 && payload.seqs.includes(seq)) {
                return { ...m, status: "read" as const };
              }
              if (hasReadSeq <= 0 && !payload.seqs?.length) {
                return { ...m, status: "read" as const };
              }
              return m;
            }),
          },
        };
      });
    },
    [user?.userId]
  );

  const applyRevokeLocal = useCallback(
    (conversationId: string, clientMsgId?: string, seq?: number) => {
      const revokedContent = "撤回了一条消息";
      setState((prev) => {
        const convMessages = prev.messages[conversationId];
        if (!convMessages) return prev;
        let hit: Message | undefined;
        const nextMsgs = convMessages.map((m) => {
          const match =
            (clientMsgId && (m.clientMsgId === clientMsgId || m.messageId === clientMsgId)) ||
            (seq && m.seq === seq);
          if (!match) return m;
          hit = m;
          return {
            ...m,
            type: "system" as const,
            status: "revoked" as const,
            content: revokedContent,
            file: undefined,
          };
        });
        if (!hit) return prev;
        const conversations = prev.conversations.map((c) => {
          if (c.conversationId !== conversationId) return c;
          const lastId = c.lastMessage?.messageId;
          const lastClient = c.lastMessage?.clientMsgId;
          if (
            lastId === hit?.messageId ||
            lastClient === hit?.clientMsgId ||
            (seq && c.lastMessage?.seq === seq)
          ) {
            return {
              ...c,
              lastMessage: {
                ...(c.lastMessage as Message),
                type: "system" as const,
                status: "revoked" as const,
                content: revokedContent,
              },
            };
          }
          return c;
        });
        return {
          ...prev,
          messages: { ...prev.messages, [conversationId]: nextMsgs },
          conversations,
        };
      });
      if (user?.userId) {
        void getIdb(user.userId)
          .upsertMessages([
            {
              messageId: clientMsgId || `revoked_${seq || Date.now()}`,
              clientMsgId,
              conversationId,
              senderId: "",
              senderName: "",
              senderAvatar: "",
              content: revokedContent,
              type: "system",
              status: "revoked",
              createdAt: new Date().toISOString(),
              seq,
            },
          ])
          .catch(() => undefined);
      }
    },
    [user?.userId]
  );

  const handleMessageRevoke = useCallback(
    (wsMsg: WsMessage) => {
      const payload = wsMsg.payload as {
        conversationId: string;
        clientMsgId?: string;
        seq?: number;
      };
      if (!payload?.conversationId) return;
      applyRevokeLocal(payload.conversationId, payload.clientMsgId, payload.seq);
    },
    [applyRevokeLocal]
  );

  const handleUserStatus = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as { userId: string; status: User["status"] };
    if (!payload?.userId) return;
    setState((prev) => ({
      ...prev,
      contacts: prev.contacts.map((c) =>
        c.userId === payload.userId ? { ...c, status: payload.status } : c
      ),
    }));
  }, []);

  const handleTyping = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as {
      conversationId: string;
      userId: string;
      isTyping: boolean;
    };
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

  const handleSyncCompleted = useCallback((wsMsg: WsMessage) => {
    const payload = wsMsg.payload as {
      conversationIds: string[];
      messagesByConversation: Record<string, Message[]>;
    };
    setState((prev) => {
      let nextMessages = prev.messages;
      let changed = false;
      for (const convId of payload.conversationIds) {
        if (convId !== prev.activeConversationId) continue;
        const incoming = payload.messagesByConversation[convId] || [];
        if (!incoming.length) continue;
        const existing = nextMessages[convId] || [];
        const merged = [...existing];
        for (const m of incoming) {
          if (!merged.some((x) => sameMessage(x, m) || (x.seq && x.seq === m.seq))) {
            merged.push(m);
            changed = true;
          }
        }
        merged.sort((a, b) => (a.seq || 0) - (b.seq || 0));
        nextMessages = { ...nextMessages, [convId]: merged };
      }
      return changed ? { ...prev, messages: nextMessages } : prev;
    });
  }, []);

  useEffect(() => {
    const unsubs: (() => void)[] = [];
    if (isMockMode) return;
    if (user) {
      unsubs.push(IMSDK.on("message.new", handleNewMessage));
      unsubs.push(IMSDK.on("message.read", handleReadReceipt));
      unsubs.push(IMSDK.on("message.revoke", handleMessageRevoke));
      unsubs.push(IMSDK.on("user.status", handleUserStatus));
      unsubs.push(IMSDK.on("typing", handleTyping));
      unsubs.push(IMSDK.on("sync.completed", handleSyncCompleted));
      unsubs.push(
        IMSDK.on("friend.request", () => {
          void refreshFriendRequestBadge();
          void refreshConversations();
        })
      );
      unsubs.push(
        IMSDK.on("friend.synced", (msg) => {
          const payload = msg.payload as { friends?: Contact[] } | undefined;
          if (payload?.friends) applySyncedFriends(payload.friends);
          else void refreshContacts();
        })
      );
      unsubs.push(
        IMSDK.on("group.synced", (msg) => {
          const payload = msg.payload as { groups?: Group[] } | undefined;
          if (payload?.groups) applySyncedGroups(payload.groups);
          else void refreshGroups();
        })
      );
      unsubs.push(
        IMSDK.onStatusChange((connected) => {
          setState((s) => ({ ...s, wsConnected: connected }));
        })
      );
      unsubs.push(IMSDK.on("call.invite", handleCallInvite));
      unsubs.push(IMSDK.on("call.accepted", (msg) => void handleCallAccepted(msg)));
      unsubs.push(IMSDK.on("call.rejected", (msg) => void handleCallEndedLike(msg)));
      unsubs.push(IMSDK.on("call.cancelled", (msg) => void handleCallEndedLike(msg)));
      unsubs.push(IMSDK.on("call.timeout", (msg) => void handleCallEndedLike(msg)));
      unsubs.push(IMSDK.on("call.busy", (msg) => void handleCallEndedLike(msg)));
      unsubs.push(IMSDK.on("call.ended", (msg) => void handleCallEndedLike(msg)));
    }
    return () => unsubs.forEach((fn) => fn());
  }, [
    user,
    handleNewMessage,
    handleReadReceipt,
    handleMessageRevoke,
    handleUserStatus,
    handleTyping,
    handleSyncCompleted,
    refreshFriendRequestBadge,
    refreshConversations,
    refreshContacts,
    applySyncedFriends,
    refreshGroups,
    applySyncedGroups,
    handleCallInvite,
    handleCallAccepted,
    handleCallEndedLike,
  ]);

  const loadMessages = useCallback(
    async (conversationId: string) => {
      if (isMockMode) return;
      try {
        const userId = user?.userId;
        let local: Message[] = [];
        if (userId) {
          try {
            local = await getIdb(userId).getMessagesByConversation(conversationId, 50);
          } catch {
            local = [];
          }
        }
        const msgs = await IMSDK.getAdvancedHistoryMessageList(conversationId, { limit: 50 });
        if (userId && msgs.length) {
          void getIdb(userId).upsertMessages(msgs);
        }
        setState((prev) => {
          const existing = prev.messages[conversationId] || [];
          const pending = existing.filter((m) => m.status === "sending" || m.status === "failed");
          const merged = [...local];
          for (const m of msgs) {
            const idx = merged.findIndex(
              (x) => sameMessage(x, m) || (!!x.seq && !!m.seq && x.seq === m.seq)
            );
            if (idx >= 0) {
              merged[idx] = m;
            } else {
              merged.push(m);
            }
          }
          for (const p of pending) {
            if (!merged.some((m) => sameMessage(m, p))) merged.push(p);
          }
          merged.sort(
            (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()
          );
          return {
            ...prev,
            messages: { ...prev.messages, [conversationId]: merged },
          };
        });
      } catch {
        // keep current
      }
    },
    [user?.userId]
  );

  const markConversationRead = useCallback((conversationId: string) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.map((c) =>
        c.conversationId === conversationId ? { ...c, unreadCount: 0 } : c
      ),
    }));
    if (isMockMode) return;
    const msgs = stateRef.current.messages[conversationId] || [];
    const maxSeq = msgs.reduce((max, m) => Math.max(max, m.seq || 0), 0);
    if (maxSeq <= 0) return;
    void IMSDK.markConversationMessageAsRead(conversationId, maxSeq).catch(() => undefined);
  }, []);

  const revokeMessage = useCallback(
    async (conversationId: string, clientMsgId: string) => {
      if (!clientMsgId) return;
      if (isMockMode) {
        applyRevokeLocal(conversationId, clientMsgId);
        return;
      }
      await IMSDK.revokeMessage(conversationId, clientMsgId);
      applyRevokeLocal(conversationId, clientMsgId);
    },
    [applyRevokeLocal]
  );

  const setActiveConversation = useCallback((id: string) => {
    storage.setActiveConversationId(id);
    setState((prev) => ({
      ...prev,
      activeConversationId: id,
      conversations: prev.conversations.map((c) =>
        c.conversationId === id ? { ...c, unreadCount: 0 } : c
      ),
    }));
  }, []);

  const sendMessage = useCallback(
    async (req: SendMessageRequest): Promise<Message> => {
      const clientMsgId = `msg_local_${Date.now()}`;
      const localMsg: Message = {
        messageId: clientMsgId,
        clientMsgId,
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

      setState((prev) => ({
        ...prev,
        messages: {
          ...prev.messages,
          [req.conversationId]: [...(prev.messages[req.conversationId] || []), localMsg],
        },
        conversations: prev.conversations.map((c) =>
          c.conversationId === req.conversationId
            ? { ...c, lastMessage: localMsg, updatedAt: localMsg.createdAt }
            : c
        ),
      }));

      if (isMockMode) {
        await new Promise((resolve) => setTimeout(resolve, 280));
        const sent = { ...localMsg, status: "sent" as const };
        setState((prev) => ({
          ...prev,
          messages: {
            ...prev.messages,
            [req.conversationId]: (prev.messages[req.conversationId] || []).map((m) =>
              m.messageId === clientMsgId ? sent : m
            ),
          },
        }));
        return sent;
      }

      const markFailed = (): Message => {
        const failedMsg: Message = { ...localMsg, status: "failed" };
        setState((prev) => ({
          ...prev,
          messages: {
            ...prev.messages,
            [req.conversationId]: (prev.messages[req.conversationId] || []).map((m) =>
              m.messageId === clientMsgId || m.clientMsgId === clientMsgId ? failedMsg : m
            ),
          },
          conversations: prev.conversations.map((c) =>
            c.conversationId === req.conversationId
              ? { ...c, lastMessage: failedMsg, updatedAt: failedMsg.createdAt }
              : c
          ),
        }));
        return failedMsg;
      };

      try {
        const conversation = stateRef.current.conversations.find(
          (item) => item.conversationId === req.conversationId
        );
        const content = req.file
          ? JSON.stringify({
              file_id: req.file.fileId,
              name: req.file.name,
              content_type: req.file.contentType,
              size: req.file.size,
              sha256: req.file.sha256,
              category: req.file.category,
              expires_at: req.file.expiresAt,
            })
          : req.content;
        const result = await IMSDK.sendMessage({
          clientMsgId,
          conversationId: req.conversationId,
          sessionType: conversation?.type === "group" ? 2 : 1,
          groupId:
            conversation?.type === "group"
              ? IMSDK.parseGroupId(conversation.groupId || req.conversationId)
              : "",
          recvId:
            conversation?.type === "private"
              ? conversation.members.find((member) => member.userId !== user?.userId)?.userId ||
                ""
              : "",
          recvUserIds:
            conversation?.members
              .filter((member) => member.userId !== user?.userId)
              .map((member) => member.userId) || [],
          contentType: req.type === "image" ? 1 : req.type === "file" ? 2 : 0,
          content,
          senderNickname: user?.displayName || user?.username || "",
          senderFaceUrl: user?.avatar || "",
        });
        const sentMsg: Message = {
          ...localMsg,
          messageId: result.serverMsgId,
          clientMsgId,
          seq: result.seq || undefined,
          status: "sent",
          createdAt: (() => {
            const ms = toEpochMs(result.sendTime);
            return Number.isFinite(ms) ? new Date(ms).toISOString() : localMsg.createdAt;
          })(),
        };
        setState((prev) => ({
          ...prev,
          messages: {
            ...prev.messages,
            [req.conversationId]: (prev.messages[req.conversationId] || []).map((m) =>
              m.messageId === clientMsgId || m.clientMsgId === clientMsgId ? sentMsg : m
            ),
          },
          conversations: prev.conversations.map((c) =>
            c.conversationId === req.conversationId
              ? { ...c, lastMessage: sentMsg, updatedAt: sentMsg.createdAt }
              : c
          ),
        }));
        if (user?.userId && sentMsg.clientMsgId) {
          void getIdb(user.userId).upsertMessages([sentMsg]);
        }
        return sentMsg;
      } catch (error) {
        // 拉黑 / 无权限等发送失败：气泡标 failed，不向上抛以免打断 UI
        if (process.env.NODE_ENV === "development") {
          console.warn("[ChatContext] sendMessage failed:", error);
        }
        return markFailed();
      }
    },
    [user]
  );

  const sendTyping = useCallback(
    (conversationId: string, isTyping: boolean) => {
      IMSDK.send("typing", {
        conversationId,
        userId: user?.userId,
        isTyping,
      });
    },
    [user]
  );

  const addConversation = useCallback((conv: Conversation) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.some((item) => item.conversationId === conv.conversationId)
        ? prev.conversations.map((item) =>
            item.conversationId === conv.conversationId ? conv : item
          )
        : [conv, ...prev.conversations],
    }));
  }, []);

  const removeConversation = useCallback((id: string) => {
    setState((prev) => ({
      ...prev,
      conversations: prev.conversations.filter((c) => c.conversationId !== id),
      activeConversationId: prev.activeConversationId === id ? null : prev.activeConversationId,
    }));
  }, []);

  const searchContacts = useCallback(
    (query: string): Contact[] => {
      if (!query.trim()) return state.contacts;
      const q = query.toLowerCase();
      return state.contacts.filter(
        (c) =>
          c.displayName.toLowerCase().includes(q) || c.username.toLowerCase().includes(q)
      );
    },
    [state.contacts]
  );

  const createGroup = useCallback(
    async (name: string, memberIds: string[]): Promise<Conversation | null> => {
      if (isMockMode) {
        const groupId = `mock_${Date.now()}`;
        const conversationId = `gid_${groupId}`;
        const newConv: Conversation = {
          conversationId,
          type: "group",
          groupId,
          title: name,
          avatar: "",
          unreadCount: 0,
          isPinned: false,
          isMuted: false,
          members: [
            {
              userId: user?.userId || "",
              conversationId,
              role: "owner",
              joinedAt: new Date().toISOString(),
            },
            ...memberIds.map((id) => ({
              userId: id,
              conversationId,
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
              groupId,
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

      // OpenIM 风格：CreateGroup 返回 Group；会话由服务端联动创建后刷新列表
      const group = await IMSDK.createGroup({ name, memberIds });
      const conversationId = IMSDK.groupConversationId(group.groupId);
      const [convs, groups] = await Promise.all([
        IMSDK.getAllConversationList(),
        IMSDK.getJoinedGroupList().catch(() => [
          group,
          ...stateRef.current.groups.filter((g) => g.groupId !== group.groupId),
        ]),
      ]);
      const enriched = enrichConversations(
        convs,
        stateRef.current.contacts,
        user?.userId,
        [group, ...stateRef.current.groups.filter((g) => g.groupId !== group.groupId)]
      ).map((conv) => {
        if (conv.conversationId !== conversationId) return conv;
        return {
          ...conv,
          title: conv.title || group.name,
          avatar: conv.avatar || group.avatar,
        };
      });
      setState((prev) => ({
        ...prev,
        conversations: enriched,
        groups: groups.some((g) => g.groupId === group.groupId)
          ? groups
          : [group, ...groups.filter((g) => g.groupId !== group.groupId)],
      }));

      const found = enriched.find((c) => c.conversationId === conversationId);
      if (found) {
        return {
          ...found,
          groupId: found.groupId || group.groupId,
          title: found.title || group.name,
          avatar: found.avatar || group.avatar,
        };
      }

      return {
        conversationId,
        type: "group",
        groupId: group.groupId,
        title: group.name,
        avatar: group.avatar,
        unreadCount: 0,
        members: [],
        isPinned: false,
        isMuted: false,
        createdAt: group.createdAt,
        updatedAt: group.createdAt,
      };
    },
    [user]
  );

  const openOrCreatePrivateChat = useCallback(
    async (peerUserId: string): Promise<string | null> => {
      const existing = stateRef.current.conversations.find(
        (item) =>
          item.type === "private" && item.members.some((member) => member.userId === peerUserId)
      );
      if (existing) {
        setActiveConversation(existing.conversationId);
        return existing.conversationId;
      }
      try {
        await IMSDK.createPrivateConversation(peerUserId);
        const convs = await IMSDK.getAllConversationList();
        const enriched = enrichConversations(
          convs,
          stateRef.current.contacts,
          user?.userId,
          stateRef.current.groups
        );
        setState((prev) => ({ ...prev, conversations: enriched }));
        const found = enriched.find(
          (item) =>
            item.type === "private" && item.members.some((member) => member.userId === peerUserId)
        );
        if (found) {
          setActiveConversation(found.conversationId);
          return found.conversationId;
        }
      } catch {
        // ignore
      }
      return null;
    },
    [setActiveConversation, user?.userId]
  );

  return (
    <ChatContext.Provider
      value={{
        ...state,
        setActiveConversation,
        sendMessage,
        sendTyping,
        markConversationRead,
        revokeMessage,
        addConversation,
        removeConversation,
        updateConversation,
        refreshConversations,
        refreshGroups,
        loadMessages,
        searchContacts,
        createGroup,
        refreshFriendRequestBadge,
        refreshContacts,
        openOrCreatePrivateChat,
        callPhase,
        callSession,
        callBusy,
        callError,
        startVoiceCall,
        acceptCall,
        rejectCall,
        cancelCall,
        hangupCall,
        toggleCallMute,
        dismissCallError,
      }}
    >
      {children}
      {!isMockMode ? (
        <CallOverlay
          phase={callPhase}
          peerName={callSession?.peerName || ""}
          peerAvatar={callSession?.peerAvatar || ""}
          muted={callSession?.muted ?? false}
          busy={callBusy}
          error={callError}
          activeSince={callSession?.activeSince}
          onAccept={() => void acceptCall()}
          onReject={() => void rejectCall()}
          onCancel={() => void cancelCall()}
          onHangup={() => void hangupCall()}
          onToggleMute={() => void toggleCallMute()}
          onDismissError={dismissCallError}
        />
      ) : null}
    </ChatContext.Provider>
  );
}

export function useChat(): ChatContextValue {
  const ctx = useContext(ChatContext);
  if (!ctx) throw new Error("useChat must be used within ChatProvider");
  return ctx;
}

function enrichConversations(
  conversations: Conversation[],
  contacts: Contact[],
  currentUserId?: string,
  groups: Group[] = []
): Conversation[] {
  const contactMap = new Map(contacts.map((c) => [c.userId, c]));
  const groupMap = new Map(groups.map((g) => [g.groupId, g]));
  return conversations.map((conv) => {
    if (conv.type === "group") {
      const groupId = conv.groupId || IMSDK.parseGroupId(conv.conversationId);
      const group = groupId ? groupMap.get(groupId) : undefined;
      return {
        ...conv,
        groupId: groupId || undefined,
        // 会话表不含群名，用已加入群列表补全标题/头像
        title: conv.title || group?.name || "",
        avatar: conv.avatar || group?.avatar || "",
      };
    }
    if (conv.type !== "private") return conv;
    const peerId =
      conv.members.find((m) => m.userId !== currentUserId)?.userId ||
      conv.members[0]?.userId ||
      "";
    const contact = peerId ? contactMap.get(peerId) : undefined;
    // BFF 已填 title/avatar 时优先保留；本地好友列表仅作兜底
    return {
      ...conv,
      title: conv.title || contact?.displayName || peerId || "",
      avatar: conv.avatar || contact?.avatar || "",
    };
  });
}

function sameMessage(a: Message, b: Message): boolean {
  if (a.messageId && b.messageId && a.messageId === b.messageId) return true;
  const aClient = a.clientMsgId || (a.messageId.startsWith("msg_local_") ? a.messageId : "");
  const bClient = b.clientMsgId || (b.messageId.startsWith("msg_local_") ? b.messageId : "");
  if (aClient && bClient && aClient === bClient) return true;
  if (a.clientMsgId && a.clientMsgId === b.messageId) return true;
  if (b.clientMsgId && b.clientMsgId === a.messageId) return true;
  return false;
}
