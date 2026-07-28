"use client";

// ============================================================
// AddFriendPanel — 搜索用户并发送好友请求
// ============================================================
import React, { useState, useCallback } from "react";
import { Search, UserPlus, Check, Clock, Loader2 } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";
import type { User, SearchedUser } from "@/types";

interface AddFriendPanelProps {
  embedded?: boolean; // 嵌入在联系人面板中时隐藏标题头
}

export default function AddFriendPanel({ embedded = false }: AddFriendPanelProps) {
  const { user: currentUser } = useAuth();
  const [searchQuery, setSearchQuery] = useState("");
  const [results, setResults] = useState<SearchedUser[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [sendingId, setSendingId] = useState<string | null>(null);
  const [sentSet, setSentSet] = useState<Set<string>>(new Set());
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const handleSearch = useCallback(async (query: string) => {
    setSearchQuery(query);
    setFeedback(null);
    if (!query.trim()) {
      setResults([]);
      return;
    }

    setIsSearching(true);
    try {
      const api = await import("@/services/api");
      const users = await api.searchUsers(query);
      console.log("[AddFriend] search results:", users.length, users);
      const enriched: SearchedUser[] = users
        .filter((u) => u.userId !== currentUser?.userId)
        .map((u) => ({
          ...u,
          isFriend: false,
          hasSentRequest: sentSet.has(u.userId),
          hasIncomingRequest: false,
        }));
      setResults(enriched);
    } catch (err) {
      console.error("[AddFriend] search error:", err);
      setFeedback({ type: "error", message: "搜索失败，请检查网络连接" });
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  }, [currentUser?.userId, sentSet]);

  const handleSendRequest = useCallback(async (targetUser: User) => {
    if (sendingId || !currentUser) return;
    setSendingId(targetUser.userId);
    setFeedback(null);

    try {
      const api = await import("@/services/api");
      await api.sendFriendRequest(targetUser.userId, "");
      setSentSet((prev) => new Set(prev).add(targetUser.userId));
      setFeedback({ type: "success", message: `已向 ${targetUser.displayName} 发送好友请求` });
      // 更新结果列表
      setResults((prev) =>
        prev.map((u) =>
          u.userId === targetUser.userId ? { ...u, hasSentRequest: true } : u
        )
      );
    } catch {
      setFeedback({ type: "error", message: "发送失败，请检查后端服务" });
    } finally {
      setSendingId(null);
    }
  }, [sendingId, currentUser]);

  return (
    <div className={embedded ? "h-full flex flex-col bg-white" : "h-full flex flex-col bg-white"}>
      {/* 头部 — 仅在独立模式显示 */}
      {!embedded && (
        <div className="px-4 py-4 border-b border-gray-100">
          <h3 className="text-base font-semibold text-gray-900">添加好友</h3>
          <p className="text-xs text-gray-400 mt-0.5">通过用户ID搜索并添加好友</p>
        </div>
      )}

      {/* 搜索框 */}
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

      {/* 反馈消息 */}
      {feedback && (
        <div
          className={cn(
            "mx-4 px-3 py-2 rounded-lg text-sm mb-2",
            feedback.type === "success"
              ? "bg-green-50 text-green-700"
              : "bg-red-50 text-red-600"
          )}
        >
          {feedback.message}
        </div>
      )}

      {/* 搜索结果 */}
      <div className="flex-1 overflow-y-auto">
        {isSearching ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
            <span className="ml-2 text-sm text-gray-400">搜索中...</span>
          </div>
        ) : !searchQuery.trim() ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400">
            <Search className="w-12 h-12 mb-3 opacity-30" />
            <p className="text-sm">输入用户ID搜索用户</p>
          </div>
        ) : results.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400">
            <UserPlus className="w-12 h-12 mb-3 opacity-30" />
            <p className="text-sm">未找到匹配的用户</p>
            <p className="text-xs mt-1 opacity-60">试试其他关键词</p>
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
                  <h4 className="text-sm font-medium text-gray-900 truncate">
                    {user.displayName}
                  </h4>
                  <p className="text-xs text-gray-400 truncate">
                    @{user.username}
                    {user.email ? ` · ${user.email}` : ""}
                  </p>
                </div>
                {user.hasSentRequest ? (
                  <span
                    className="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-gray-500
                      bg-gray-100 rounded-lg cursor-default"
                  >
                    <Clock className="w-3 h-3" />
                    已申请
                  </span>
                ) : user.hasIncomingRequest ? (
                  <span
                    className="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-indigo-600
                      bg-indigo-50 rounded-lg cursor-default"
                  >
                    他申请了你
                  </span>
                ) : (
                  <button
                    onClick={() => handleSendRequest(user)}
                    disabled={sendingId === user.userId}
                    className={cn(
                      "inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg transition-all",
                      "bg-indigo-500 text-white hover:bg-indigo-600 active:scale-95",
                      "shadow-sm shadow-indigo-200",
                      sendingId === user.userId && "opacity-60 cursor-not-allowed"
                    )}
                  >
                    {sendingId === user.userId ? (
                      <>
                        <Loader2 className="w-3 h-3 animate-spin" />
                        发送中
                      </>
                    ) : (
                      <>
                        <UserPlus className="w-3 h-3" />
                        添加
                      </>
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
