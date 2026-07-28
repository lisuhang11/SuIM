"use client";

// ============================================================
// ContactsPanel — 联系人面板（简洁版）
// 两个回弹按钮：接收到的好友申请 / 已发送的好友申请
// 下方联系人列表
// ============================================================
import React, { useState, useEffect } from "react";
import {
  UserPlus, MessageSquare, Check, X,
  UserCheck, Send, Loader2, Clock,
  ChevronRight, Search,
} from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { useAuth } from "@/contexts/AuthContext";
import type { Contact } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { getStatusText } from "@/data/mock";
import { cn, formatTime } from "@/lib/utils";
import type { SearchedUser } from "@/types";
import { mockUsers } from "@/data/mock";

interface ContactsPanelProps {
  pendingRequestCount?: number;
}

type PanelView = "none" | "requests" | "sent" | "addFriend";

export default function ContactsPanel({ pendingRequestCount = 0 }: ContactsPanelProps) {
  const {
    contacts,
    conversations,
    friendRequests,
    setActiveConversation,
    acceptFriendRequest,
    declineFriendRequest,
  } = useChat();
  const { user: currentUser } = useAuth();
  const [panelView, setPanelView] = useState<PanelView>("none");
  const [sentRequests, setSentRequests] = useState<{ toUserId: string; toNickname: string; message: string; status: number; createdAt: string }[]>([]);
  const [loadingSent, setLoadingSent] = useState(false);
  const [bouncingBtn, setBouncingBtn] = useState<string | null>(null);
  // 添加好友搜索状态
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchedUser[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [sendingId, setSendingId] = useState<string | null>(null);
  const [sentSet, setSentSet] = useState<Set<string>>(new Set());

  useEffect(() => {
    if (panelView === "sent") {
      setLoadingSent(true);
      import("@/services/api").then((api) =>
        api.getOutgoingRequests(20, 0, [0]).then((data) => {
          setSentRequests(data);
          setLoadingSent(false);
        }).catch(() => setLoadingSent(false))
      );
    }
  }, [panelView]);

  const sorted = [...contacts].sort((a, b) => {
    const order: Record<string, number> = { online: 0, away: 1, busy: 2, offline: 3 };
    return (order[a.status] ?? 4) - (order[b.status] ?? 4);
  });

  const handleStartChat = (contact: Contact) => {
    const existing = conversations.find(
      (c) => c.type === "private" && c.members.some((m) => m.userId === contact.userId)
    );
    if (existing) setActiveConversation(existing.conversationId);
  };

  const pendingRequests = (friendRequests || []).filter((r) => r.status === "pending");

  // 回弹动画：点击按钮时触发
  const triggerBounce = (key: string) => {
    setBouncingBtn(key);
    setTimeout(() => setBouncingBtn(null), 350);
  };

  const handlePanelToggle = (view: PanelView) => {
    triggerBounce(view);
    setPanelView((prev) => (prev === view ? "none" : view));
  };

  // ---- 添加好友搜索逻辑 ----
  const handleSearch = async (query: string) => {
    setSearchQuery(query);
    if (!query.trim()) { setSearchResults([]); return; }
    setIsSearching(true);
    try {
      const api = await import("@/services/api");
      const users = await api.searchUsers(query);
      setSearchResults(
        users
          .filter((u) => u.userId !== currentUser?.userId)
          .map((u) => ({
            ...u,
            isFriend: false,
            hasSentRequest: sentSet.has(u.userId),
            hasIncomingRequest: false,
          }))
      );
    } catch {
      const q = query.toLowerCase();
      setSearchResults(
        mockUsers
          .filter((u) => u.userId !== currentUser?.userId &&
            (u.username.toLowerCase().includes(q) || u.displayName.toLowerCase().includes(q)))
          .map((u) => ({ ...u, isFriend: false, hasSentRequest: sentSet.has(u.userId), hasIncomingRequest: false }))
      );
    } finally {
      setIsSearching(false);
    }
  };

  const handleSendRequest = async (target: SearchedUser) => {
    if (sendingId) return;
    setSendingId(target.userId);
    try {
      const api = await import("@/services/api");
      await api.sendFriendRequest(currentUser?.userId || "", target.userId, "");
      setSentSet((prev) => new Set(prev).add(target.userId));
      setSearchResults((prev) =>
        prev.map((u) => (u.userId === target.userId ? { ...u, hasSentRequest: true } : u))
      );
    } catch {
      setSentSet((prev) => new Set(prev).add(target.userId));
      setSearchResults((prev) =>
        prev.map((u) => (u.userId === target.userId ? { ...u, hasSentRequest: true } : u))
      );
    } finally {
      setSendingId(null);
    }
  };

  // 内嵌面板展开/收起动画
  const panelExpandClass = (visible: boolean) => cn(
    "overflow-hidden transition-all duration-300 ease-in-out",
    visible ? "max-h-[500px] opacity-100 mt-2" : "max-h-0 opacity-0 mt-0"
  );

  // ---- 回弹动画按钮样式 ----
  const bounceBtnClass = (key: string) => cn(
    "relative flex items-center gap-3 w-full px-4 py-2.5 rounded-xl text-sm font-medium",
    "transition-all duration-300 ease-out",
    panelView === key
      ? "bg-indigo-500 text-white shadow-sm shadow-indigo-200"
      : "bg-gray-100 text-gray-700 hover:bg-gray-200",
    bouncingBtn === key && "animate-[recoil_0.4s_ease-out]"
  );

  return (
    <div className="flex flex-col h-full">
      {/* ====== 两个操作按钮 ====== */}
      <div className="flex-shrink-0 px-3 pt-3 pb-2 space-y-1.5">
        {/* 接收到的好友申请 */}
        <div>
          <button
            onClick={() => handlePanelToggle("requests")}
            className={bounceBtnClass("requests")}
          >
            <UserCheck className="w-4 h-4 flex-shrink-0" />
            <span className="flex-1 text-left">接收到的好友申请</span>
            {pendingRequestCount > 0 && (
              <span className={cn(
                "min-w-[20px] h-[20px] text-[10px] font-bold rounded-full flex items-center justify-center px-1",
                panelView === "requests" ? "bg-white text-indigo-500" : "bg-red-500 text-white"
              )}>
                {pendingRequestCount > 9 ? "9+" : pendingRequestCount}
              </span>
            )}
            <ChevronRight className={cn(
              "w-4 h-4 flex-shrink-0 transition-transform duration-300",
              panelView === "requests" && "rotate-90"
            )} />
          </button>

          {/* 内嵌：接收到的好友申请列表 */}
          <div className={panelExpandClass(panelView === "requests")}>
            <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              {pendingRequests.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-6 text-gray-400 text-xs">
                  <UserCheck className="w-5 h-5 mb-1.5 opacity-30" />
                  <p>暂无接收到的好友申请</p>
                </div>
              ) : (
                <div>
                  <div className="px-3 py-1.5 flex items-center gap-2 bg-gray-50/50 border-b border-gray-50">
                    <span className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">待处理</span>
                    <span className="bg-red-500 text-white text-[10px] font-bold min-w-[16px] h-[16px] rounded-full flex items-center justify-center px-1">{pendingRequests.length}</span>
                  </div>
                  {pendingRequests.map((req) => (
                    <div key={req.requestId} className="px-3 py-2 hover:bg-gray-50 transition-colors duration-150 border-t border-gray-50">
                      <div className="flex items-center gap-2">
                        <UserAvatar name={req.fromDisplayName} size="sm" />
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center justify-between">
                            <h4 className="text-xs font-medium text-gray-900 truncate">{req.fromDisplayName}</h4>
                            <span className="text-[10px] text-gray-400 flex-shrink-0 ml-1">{formatTime(req.createdAt)}</span>
                          </div>
                          {req.message && <p className="text-[11px] text-gray-500 truncate mt-0.5">{req.message}</p>}
                          <div className="flex gap-1.5 mt-1.5">
                            <button onClick={() => acceptFriendRequest(req.requestId)}
                              className="flex items-center gap-1 px-2 py-0.5 text-[10px] font-medium bg-indigo-500 text-white rounded-md hover:bg-indigo-600 transition-all duration-200 active:scale-95">
                              <Check className="w-2.5 h-2.5" />接受</button>
                            <button onClick={() => declineFriendRequest(req.requestId)}
                              className="flex items-center gap-1 px-2 py-0.5 text-[10px] font-medium bg-gray-200 text-gray-600 rounded-md hover:bg-gray-300 transition-all duration-200 active:scale-95">
                              <X className="w-2.5 h-2.5" />拒绝</button>
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 已发送的好友申请 */}
        <div>
          <button
            onClick={() => handlePanelToggle("sent")}
            className={bounceBtnClass("sent")}
          >
            <Send className="w-4 h-4 flex-shrink-0" />
            <span className="flex-1 text-left">已发送的好友申请</span>
            <ChevronRight className={cn(
              "w-4 h-4 flex-shrink-0 transition-transform duration-300",
              panelView === "sent" && "rotate-90"
            )} />
          </button>

          {/* 内嵌：已发送的好友申请列表 */}
          <div className={panelExpandClass(panelView === "sent")}>
            <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
              {loadingSent ? (
                <div className="flex items-center justify-center py-6"><Loader2 className="w-4 h-4 text-indigo-500 animate-spin" /></div>
              ) : sentRequests.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-6 text-gray-400 text-xs">
                  <Send className="w-5 h-5 mb-1.5 opacity-30" />
                  <p>暂无已发送的好友申请</p>
                </div>
              ) : (
                <div>
                  <div className="px-3 py-1.5 flex items-center gap-2 bg-gray-50/50 border-b border-gray-50">
                    <span className="text-[10px] font-semibold text-gray-500 uppercase tracking-wider">已发送的好友申请 ({sentRequests.length})</span>
                  </div>
                  {sentRequests.map((req, idx) => (
                    <div key={idx} className="flex items-center gap-2 px-3 py-2 hover:bg-gray-50 transition-colors duration-150 border-t border-gray-50">
                      <UserAvatar name={req.toNickname} size="sm" />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between">
                          <h4 className="text-xs font-medium text-gray-900 truncate">{req.toNickname}</h4>
                          <span className="text-[10px] text-gray-400 flex-shrink-0 ml-1">{formatTime(req.createdAt)}</span>
                        </div>
                        <p className="text-[11px] text-gray-500 truncate mt-0.5">{req.message || "请求添加为好友"}</p>
                        <span className="inline-flex items-center gap-1 mt-1 text-[10px] text-amber-500">
                          <Clock className="w-2.5 h-2.5" />等待对方确认
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 添加好友搜索 */}
        <button
          onClick={() => handlePanelToggle("addFriend")}
          className={bounceBtnClass("addFriend")}
        >
          <UserPlus className="w-4 h-4 flex-shrink-0" />
          <span className="flex-1 text-left">添加好友</span>
          <ChevronRight className={cn(
            "w-4 h-4 flex-shrink-0 transition-transform duration-300",
            panelView === "addFriend" && "rotate-90"
          )} />
        </button>

        {/* 内嵌：搜索添加好友 */}
        <div className={panelExpandClass(panelView === "addFriend")}>
          <div className="bg-white rounded-xl border border-gray-100 overflow-hidden">
            {/* 搜索框 */}
            <div className="relative px-3 pt-2.5 pb-2">
              <Search className="absolute left-5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => handleSearch(e.target.value)}
                placeholder="输入用户 ID 搜索..."
                className="w-full pl-8 pr-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-lg
                  focus:bg-white focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 outline-none transition-all"
              />
            </div>

            {/* 搜索结果 */}
            <div className={cn(
              "overflow-hidden transition-all duration-250 ease-in-out",
              searchQuery.trim() ? "max-h-60" : "max-h-0"
            )}>
              {isSearching ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="w-4 h-4 text-indigo-500 animate-spin" />
                </div>
              ) : searchResults.length === 0 && searchQuery.trim() ? (
                <div className="flex flex-col items-center justify-center py-4 text-gray-400 text-xs border-t border-gray-50">
                  <UserPlus className="w-5 h-5 mb-1 opacity-30" />
                  <p>未找到匹配的用户</p>
                </div>
              ) : (
                <div className="border-t border-gray-50 max-h-48 overflow-y-auto">
                  {searchResults.map((u) => (
                    <div key={u.userId} className="flex items-center gap-2 px-3 py-2 hover:bg-gray-50 transition-colors duration-150 border-b border-gray-50 last:border-b-0">
                      <UserAvatar name={u.displayName} size="sm" />
                      <div className="flex-1 min-w-0">
                        <h4 className="text-xs font-medium text-gray-900 truncate">{u.displayName}</h4>
                        <p className="text-[10px] text-gray-400 truncate">@{u.username}</p>
                      </div>
                      {u.hasSentRequest ? (
                        <span className="text-[10px] text-gray-400 bg-gray-100 px-2 py-0.5 rounded-md flex-shrink-0">已申请</span>
                      ) : (
                        <button
                          onClick={() => handleSendRequest(u)}
                          disabled={sendingId === u.userId}
                          className={cn(
                            "text-[10px] font-medium px-2.5 py-1 rounded-md flex-shrink-0 transition-all duration-200",
                            "bg-indigo-500 text-white hover:bg-indigo-600 active:scale-95",
                            sendingId === u.userId && "opacity-50"
                          )}
                        >
                          {sendingId === u.userId ? "发送中" : "添加"}
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* ====== 联系人列表 ====== */}
      <div className="flex-1 overflow-y-auto">
        {sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-gray-400 text-sm">
            <UserPlus className="w-10 h-10 mb-3 opacity-30" />
            <p className="font-medium">暂无联系人</p>
            <p className="text-xs mt-1">点击上方按钮添加好友</p>
          </div>
        ) : (
          sorted.map((contact) => (
            <button key={contact.userId} onClick={() => handleStartChat(contact)}
              className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 transition-colors duration-150 text-left group">
              <div className="relative flex-shrink-0">
                <UserAvatar name={contact.displayName} size="md" />
                <OnlineBadge status={contact.status} size="sm" className="absolute -bottom-0.5 -right-0.5" />
              </div>
              <div className="flex-1 min-w-0">
                <h4 className="text-sm font-medium text-gray-900 truncate">{contact.displayName}</h4>
                <p className="text-xs text-gray-400">@{contact.username} · {getStatusText(contact.status)}</p>
              </div>
              <MessageSquare className="w-4 h-4 text-gray-300 group-hover:text-indigo-400 transition-colors duration-200 flex-shrink-0" />
            </button>
          ))
        )}
      </div>
    </div>
  );
}
