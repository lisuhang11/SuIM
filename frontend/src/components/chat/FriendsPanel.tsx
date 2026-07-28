"use client";

// ============================================================
// FriendsPanel — 好友栏目（好友列表 + 添加好友 + 好友请求）
// ============================================================
import React, { useState, useEffect, useCallback } from "react";
import {
  Search,
  UserPlus,
  Bell,
  MessageSquare,
  ArrowLeft,
  MoreVertical,
  UserX,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { useAuth } from "@/contexts/AuthContext";
import type { Contact, FriendRequest } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { getStatusText } from "@/lib/status";
import { cn } from "@/lib/utils";

type SubView = "list" | "add" | "requests";

export default function FriendsPanel() {
  const { contacts, conversations, setActiveConversation, refreshConversations, searchContacts, friendRequestBadge, friendRequestVersion } = useChat();
  const { user: currentUser } = useAuth();
  const [subView, setSubView] = useState<SubView>("list");

  // ---- 好友列表 ----
  const [search, setSearch] = useState("");
  const filtered = search.trim() ? searchContacts(search) : contacts;
  const sorted = [...filtered].sort((a, b) => {
    const order: Record<string, number> = { online: 0, away: 1, busy: 2, offline: 3 };
    return (order[a.status] ?? 4) - (order[b.status] ?? 4);
  });

  const handleStartChat = async (contact: Contact) => {
    // 查找已有私聊会话
    const existing = conversations.find(
      (c) =>
        c.type === "private" &&
        c.members.some((m) => m.userId === contact.userId)
    );
    if (existing) {
      setActiveConversation(existing.conversationId);
      return;
    }

    // 自动创建私聊会话
    try {
      const api = await import("@/services/api");
      const conv = await api.createPrivateConversation(contact.userId);
      refreshConversations();
      if (conv?.conversationId) {
        setActiveConversation(conv.conversationId);
      }
    } catch {
      // 回退：尝试再次刷新会话列表
      refreshConversations();
    }
  };

  const handleDeleteFriend = useCallback(async (friendId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    e.preventDefault();
    if (!window.confirm("确定要删除该好友吗？")) return;
    try {
      const api = await import("@/services/api");
      await api.deleteFriend(friendId);
      refreshConversations();
    } catch {
      // fallback
    }
  }, [refreshConversations]);

  return (
    <div className="h-full flex flex-col bg-white w-full">
      {/* ---- 子视图：添加好友 ---- */}
      {subView === "add" && (
        <div className="h-full flex flex-col">
          <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-100 flex-shrink-0">
            <button
              onClick={() => setSubView("list")}
              className="p-1.5 -ml-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
            </button>
            <h2 className="text-sm font-semibold text-gray-900">添加好友</h2>
          </div>
          <div className="flex-1 overflow-y-auto">
            <SearchAndAdd onBack={() => setSubView("list")} currentUser={currentUser} />
          </div>
        </div>
      )}

      {/* ---- 子视图：好友请求 ---- */}
      {subView === "requests" && (
        <div className="h-full flex flex-col">
          <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-100 flex-shrink-0">
            <button
              onClick={() => { setSubView("list"); }}
              className="p-1.5 -ml-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            >
              <ArrowLeft className="w-4 h-4" />
            </button>
            <h2 className="text-sm font-semibold text-gray-900">好友通知</h2>
          </div>
          <div className="flex-1 overflow-y-auto">
            <FriendRequestPanel embedded friendRequestVersion={friendRequestVersion} />
          </div>
        </div>
      )}

      {/* ---- 默认：好友列表 ---- */}
      {subView === "list" && (
        <>
          {/* 头部 */}
          <div className="h-14 flex items-center justify-between px-4 border-b border-gray-100 flex-shrink-0">
            <h2 className="text-sm font-semibold text-gray-900">好友</h2>
            <div className="flex items-center gap-1.5">
              <button
                onClick={() => setSubView("add")}
                className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-indigo-600
                  bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors"
              >
                <UserPlus className="w-3.5 h-3.5" />
                添加
              </button>
              <button
                onClick={() => setSubView("requests")}
                className="relative p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
                title="好友通知"
              >
                <Bell className="w-4 h-4" />
                {friendRequestBadge > 0 && (
                  <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 flex items-center justify-center
                    px-1 text-[10px] font-bold text-white bg-red-500 rounded-full">
                    {friendRequestBadge > 99 ? "99+" : friendRequestBadge}
                  </span>
                )}
              </button>
            </div>
          </div>

          {/* 搜索 */}
          <div className="px-3 py-2.5">
            <div className="relative">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="搜索好友..."
                className="w-full pl-8 pr-3 py-1.5 text-xs bg-gray-100 rounded-lg
                  placeholder:text-gray-400 focus:outline-none focus:ring-1 focus:ring-indigo-300"
              />
            </div>
          </div>

          {/* 好友列表 */}
          <div className="flex-1 overflow-y-auto">
            {sorted.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
                <UserPlus className="w-8 h-8 mb-2 opacity-40" />
                <p>暂无好友</p>
                <p className="text-xs mt-1">点击"添加"按钮添加好友</p>
              </div>
            ) : (
              sorted.map((contact) => (
                <div
                  key={contact.userId}
                  onClick={() => handleStartChat(contact)}
                  className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 transition-colors cursor-pointer group"
                >
                  <div className="relative flex-shrink-0">
                    <UserAvatar name={contact.displayName} size="md" />
                    <OnlineBadge
                      status={contact.status}
                      size="sm"
                      className="absolute -bottom-0.5 -right-0.5"
                    />
                  </div>
                  <div className="flex-1 min-w-0">
                    <h4 className="text-sm font-medium text-gray-900 truncate">
                      {contact.displayName}
                    </h4>
                    <p className="text-xs text-gray-400">
                      @{contact.username} · {getStatusText(contact.status)}
                    </p>
                  </div>
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      onClick={() => handleStartChat(contact)}
                      className="p-1.5 rounded-lg hover:bg-indigo-50 text-gray-400 hover:text-indigo-600 transition-colors"
                      title="发消息"
                    >
                      <MessageSquare className="w-4 h-4" />
                    </button>
                    <button
                      onClick={(e) => handleDeleteFriend(contact.userId, e)}
                      className="p-1.5 rounded-lg hover:bg-red-50 text-gray-400 hover:text-red-500 transition-colors"
                      title="删除好友"
                    >
                      <UserX className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}

// ===== 搜索用户并发送好友请求（嵌入版） =====
function SearchAndAdd({ onBack, currentUser }: { onBack: () => void; currentUser: any }) {
  const [searchQuery, setSearchQuery] = useState("");
  const [results, setResults] = useState<any[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [sendingId, setSendingId] = useState<string | null>(null);
  const [sentSet, setSentSet] = useState<Set<string>>(new Set());
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const handleSearch = useCallback(async (query: string) => {
    setSearchQuery(query);
    setFeedback(null);
    if (!query.trim()) { setResults([]); return; }

    setIsSearching(true);
    try {
      const api = await import("@/services/api");
      const users = await api.searchUsers(query);
      setResults(
        users
          .filter((u: any) => u.userId !== currentUser?.userId)
          .map((u: any) => ({
            ...u,
            hasSentRequest: sentSet.has(u.userId),
          }))
      );
    } catch (err) {
      console.error("[FriendsPanel] search error:", err);
      setFeedback({ type: "error", message: "搜索失败，请检查网络连接" });
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  }, [currentUser?.userId, sentSet]);

  const handleSendRequest = useCallback(async (targetUser: any) => {
    if (sendingId) return;
    setSendingId(targetUser.userId);
    setFeedback(null);
    try {
      const api = await import("@/services/api");
      await api.sendFriendRequest(targetUser.userId, "");
      setSentSet((prev) => new Set(prev).add(targetUser.userId));
      setFeedback({ type: "success", message: `已向 ${targetUser.displayName} 发送好友请求` });
      setResults((prev) =>
        prev.map((u) => u.userId === targetUser.userId ? { ...u, hasSentRequest: true } : u)
      );
    } catch {
      setFeedback({ type: "error", message: "发送失败，请检查后端服务" });
    } finally {
      setSendingId(null);
    }
  }, [sendingId]);

  return (
    <div className="flex flex-col h-full bg-white">
      <div className="px-4 py-3">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => handleSearch(e.target.value)}
            placeholder="输入用户ID搜索..."
            className="w-full pl-10 pr-4 py-2.5 text-sm bg-gray-50 border border-gray-200
              rounded-xl focus:bg-white focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100
              outline-none transition-all"
          />
        </div>
      </div>

      {feedback && (
        <div className={cn(
          "mx-4 px-3 py-2 rounded-lg text-sm mb-2",
          feedback.type === "success" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-600"
        )}>
          {feedback.message}
        </div>
      )}

      <div className="flex-1 overflow-y-auto">
        {isSearching ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
            <span className="ml-2 text-sm text-gray-400">搜索中...</span>
          </div>
        ) : !searchQuery.trim() ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400">
            <Search className="w-12 h-12 mb-3 opacity-30" />
            <p className="text-sm">输入用户名搜索用户</p>
          </div>
        ) : results.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400">
            <UserPlus className="w-12 h-12 mb-3 opacity-30" />
            <p className="text-sm">未找到匹配的用户</p>
          </div>
        ) : (
          <div className="px-3 pb-3 space-y-1">
            {results.map((user) => (
              <div
                key={user.userId}
                className="flex items-center gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 transition-colors"
              >
                <UserAvatar name={user.displayName} size="md" />
                <div className="flex-1 min-w-0">
                  <h4 className="text-sm font-medium text-gray-900 truncate">{user.displayName}</h4>
                  <p className="text-xs text-gray-400 truncate">@{user.username}</p>
                </div>
                {user.hasSentRequest ? (
                  <span className="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-gray-500 bg-gray-100 rounded-lg cursor-default">
                    已申请
                  </span>
                ) : (
                  <button
                    onClick={() => handleSendRequest(user)}
                    disabled={sendingId === user.userId}
                    className={cn(
                      "inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg transition-all",
                      "bg-indigo-500 text-white hover:bg-indigo-600 active:scale-95 shadow-sm",
                      sendingId === user.userId && "opacity-60 cursor-not-allowed"
                    )}
                  >
                    {sendingId === user.userId ? (
                      <><Loader2 className="w-3 h-3 animate-spin" />发送中</>
                    ) : (
                      <><UserPlus className="w-3 h-3" />添加</>
                    )}
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ===== 好友请求管理（嵌入版） =====
function FriendRequestPanel({
  friendRequestVersion,
}: {
  friendRequestVersion: number;
  embedded?: boolean;
}) {
  type TabType = "incoming" | "outgoing";
  const { refreshConversations, refreshFriendRequestBadge } = useChat();
  const [tab, setTab] = useState<TabType>("incoming");
  const [incomingReqs, setIncomingReqs] = useState<FriendRequest[]>([]);
  const [outgoingReqs, setOutgoingReqs] = useState<FriendRequest[]>([]);
  const [loading, setLoading] = useState(false);
  const [handlingId, setHandlingId] = useState<string | null>(null);
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const loadRequests = useCallback(async () => {
    setLoading(true);
    setFeedback(null);
    try {
      const api = await import("@/services/api");
      const [incomingRes, outgoingRes] = await Promise.allSettled([
        api.getIncomingRequests({ handleResults: [0] }),
        api.getOutgoingRequests({ handleResults: [0] }),
      ]);
      if (incomingRes.status === "fulfilled") setIncomingReqs(incomingRes.value);
      if (outgoingRes.status === "fulfilled") setOutgoingReqs(outgoingRes.value);
    } catch { /* use existing */ }
    finally { setLoading(false); }
  }, []);

  // 首次加载 + WS 推送后自动刷新
  useEffect(() => { loadRequests(); }, [loadRequests, friendRequestVersion]);

  const handleRespond = useCallback(async (request: FriendRequest, accept: boolean) => {
    if (handlingId) return;
    setHandlingId(request.requestId);
    setFeedback(null);
    try {
      const api = await import("@/services/api");
      await api.respondFriendRequest(
        request.fromUserId,
        accept ? 1 : -1, accept ? "已同意" : ""
      );
      setIncomingReqs((prev) => prev.filter((r) => r.requestId !== request.requestId));
      setFeedback({
        type: "success",
        message: accept ? "已同意好友请求" : "已拒绝好友请求",
      });
      if (accept) refreshConversations();
      refreshFriendRequestBadge();
    } catch {
      setFeedback({ type: "error", message: "处理失败，请稍后再试" });
    } finally {
      setHandlingId(null);
    }
  }, [handlingId, refreshConversations, refreshFriendRequestBadge]);

  return (
    <div className="flex flex-col h-full bg-white">
      {/* 标签切换 */}
      <div className="flex border-b border-gray-100 px-4">
        <button
          onClick={() => { setTab("incoming"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "incoming" ? "text-indigo-600" : "text-gray-400 hover:text-gray-600"
          )}
        >
          收到的请求
          {incomingReqs.length > 0 && (
            <span className="ml-1.5 px-1.5 py-0.5 text-xs font-bold text-white bg-red-500 rounded-full">
              {incomingReqs.length}
            </span>
          )}
          {tab === "incoming" && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-indigo-500 rounded-full" />}
        </button>
        <button
          onClick={() => { setTab("outgoing"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "outgoing" ? "text-indigo-600" : "text-gray-400 hover:text-gray-600"
          )}
        >
          发出的请求
          {tab === "outgoing" && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-indigo-500 rounded-full" />}
        </button>
        <div className="ml-auto flex items-center">
          <button
            onClick={loadRequests}
            className={cn("p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600", loading && "animate-spin")}
            title="刷新"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {feedback && (
        <div className={cn(
          "mx-4 mt-3 px-3 py-2 rounded-lg text-sm",
          feedback.type === "success" ? "bg-green-50 text-green-700" : "bg-red-50 text-red-600"
        )}>
          {feedback.message}
        </div>
      )}

      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
            <span className="ml-2 text-sm text-gray-400">加载中...</span>
          </div>
        ) : tab === "incoming" ? (
          incomingReqs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
              <p>暂无收到的请求</p>
            </div>
          ) : (
            <div className="px-3 py-3 space-y-2">
              {incomingReqs.map((req) => (
                <div key={req.requestId} className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80">
                  <UserAvatar name={req.fromUser?.displayName || req.fromUserId} size="md" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-medium text-gray-900">{req.fromUser?.displayName || req.fromUserId}</h4>
                      <span className="text-xs text-gray-400">@{req.fromUser?.username || req.fromUserId}</span>
                    </div>
                    {req.message && <p className="text-xs text-gray-500 mt-1 line-clamp-2">{req.message}</p>}
                    <p className="text-xs text-gray-400 mt-1.5">{formatTimeAgo(req.createdAt)}</p>
                    <div className="flex gap-2 mt-2.5">
                      <button
                        onClick={() => handleRespond(req, true)}
                        disabled={handlingId === req.requestId}
                        className={cn(
                          "inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg transition-all",
                          "bg-green-500 text-white hover:bg-green-600 active:scale-95 shadow-sm",
                          handlingId === req.requestId && "opacity-60 cursor-not-allowed"
                        )}
                      >
                        {handlingId === req.requestId ? <Loader2 className="w-3 h-3 animate-spin" /> : "✓"}
                        同意
                      </button>
                      <button
                        onClick={() => handleRespond(req, false)}
                        disabled={handlingId === req.requestId}
                        className={cn(
                          "inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg transition-all",
                          "bg-white text-gray-600 border border-gray-200 hover:bg-gray-50 active:scale-95",
                          handlingId === req.requestId && "opacity-60 cursor-not-allowed"
                        )}
                      >
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )
        ) : outgoingReqs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
            <p>暂无发出的请求</p>
          </div>
        ) : (
          <div className="px-3 py-3 space-y-2">
            {outgoingReqs.map((req) => (
              <div key={req.requestId} className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80">
                <UserAvatar name={req.toUser?.displayName || req.toUserId} size="md" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h4 className="text-sm font-medium text-gray-900">{req.toUser?.displayName || req.toUserId}</h4>
                    <span className="text-xs text-gray-400">@{req.toUser?.username || req.toUserId}</span>
                  </div>
                  {req.message && <p className="text-xs text-gray-500 mt-1 line-clamp-2">{req.message}</p>}
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs text-amber-600 bg-amber-50 rounded-full">
                      等待对方回应
                    </span>
                    <span className="text-xs text-gray-400">{formatTimeAgo(req.createdAt)}</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// ---------- 时间格式化 ----------
function formatTimeAgo(dateStr: string): string {
  if (!dateStr) return "";
  const d = new Date(dateStr).getTime();
  if (isNaN(d)) return "";
  const diff = Date.now() - d;
  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`;
  return new Date(dateStr).toLocaleDateString("zh-CN");
}
