"use client";

// ============================================================
// FriendRequestsPanel — 好友请求管理（收到的/发出的）
// ============================================================
import React, { useState, useEffect, useCallback } from "react";
import { UserPlus, UserCheck, UserX, Clock, Check, X, Loader2, RefreshCw } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";
import type { FriendRequest } from "@/types";

type TabType = "incoming" | "outgoing";

export default function FriendRequestsPanel() {
  const { refreshConversations, refreshFriendRequestBadge, friendRequestVersion } = useChat();
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
    } catch {
      // API 不可用
    } finally {
      setLoading(false);
    }
  }, []);

  // 首次加载 + WS 推送后自动刷新
  useEffect(() => {
    loadRequests();
  }, [loadRequests, friendRequestVersion]);

  const handleRespond = useCallback(
    async (request: FriendRequest, accept: boolean) => {
      if (handlingId) return;
      setHandlingId(request.requestId);
      setFeedback(null);

      try {
        const api = await import("@/services/api");
        await api.respondFriendRequest(
          request.fromUserId,
          accept ? 1 : -1,
          accept ? "已同意" : ""
        );

        // 从列表移除
        setIncomingReqs((prev) => prev.filter((r) => r.requestId !== request.requestId));
        setFeedback({
          type: "success",
          message: accept
            ? `已同意${request.fromUser?.displayName || "该用户"}的好友请求`
            : "已拒绝好友请求",
        });
        if (accept) {
          refreshConversations();
        }
        refreshFriendRequestBadge();
      } catch {
        setFeedback({ type: "error", message: "处理失败，请稍后再试" });
      } finally {
        setHandlingId(null);
      }
    },
    [handlingId, refreshConversations, refreshFriendRequestBadge]
  );

  const emptyIcon =
    tab === "incoming"
      ? (incomingReqs.length === 0 ? UserCheck : undefined)
      : (outgoingReqs.length === 0 ? Clock : undefined);

  return (
    <div className="h-full flex flex-col bg-white w-full">
      {/* 头部 */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-gray-100 flex-shrink-0">
        <div>
          <h2 className="text-sm font-semibold text-gray-900">好友请求</h2>
          <p className="text-[10px] text-gray-400">
            {incomingReqs.length > 0
              ? `${incomingReqs.length} 条待处理`
              : "暂无待处理请求"}
          </p>
        </div>
        <button
          onClick={loadRequests}
          className={cn(
            "p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors",
            loading && "animate-spin"
          )}
          title="刷新"
        >
          <RefreshCw className="w-4 h-4" />
        </button>
      </div>

      {/* 标签切换 */}
      <div className="flex border-b border-gray-100 px-4">
        <button
          onClick={() => { setTab("incoming"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "incoming"
              ? "text-indigo-600"
              : "text-gray-400 hover:text-gray-600"
          )}
        >
          收到的请求
          {incomingReqs.length > 0 && (
            <span className="ml-1.5 px-1.5 py-0.5 text-xs font-bold text-white bg-red-500 rounded-full">
              {incomingReqs.length}
            </span>
          )}
          {tab === "incoming" && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-indigo-500 rounded-full" />
          )}
        </button>
        <button
          onClick={() => { setTab("outgoing"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "outgoing"
              ? "text-indigo-600"
              : "text-gray-400 hover:text-gray-600"
          )}
        >
          发出的请求
          {tab === "outgoing" && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-indigo-500 rounded-full" />
          )}
        </button>
      </div>

      {/* 反馈 */}
      {feedback && (
        <div
          className={cn(
            "mx-4 mt-3 px-3 py-2 rounded-lg text-sm",
            feedback.type === "success"
              ? "bg-green-50 text-green-700"
              : "bg-red-50 text-red-600"
          )}
        >
          {feedback.message}
        </div>
      )}

      {/* 列表 */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
            <span className="ml-2 text-sm text-gray-400">加载中...</span>
          </div>
        ) : tab === "incoming" ? (
          incomingReqs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-gray-400">
              <UserCheck className="w-12 h-12 mb-3 opacity-30" />
              <p className="text-sm">暂无收到的请求</p>
            </div>
          ) : (
            <div className="px-3 py-3 space-y-2">
              {incomingReqs.map((req) => (
                <div
                  key={req.requestId}
                  className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80"
                >
                  <UserAvatar
                    name={req.fromUser?.displayName || "用户"}
                    size="md"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-medium text-gray-900">
                        {req.fromUser?.displayName || "未知用户"}
                      </h4>
                      <span className="text-xs text-gray-400">
                        @{req.fromUser?.username || ""}
                      </span>
                    </div>
                    {req.message && (
                      <p className="text-xs text-gray-500 mt-1 line-clamp-2">
                        {req.message}
                      </p>
                    )}
                    <p className="text-xs text-gray-400 mt-1.5">
                      {formatTimeAgo(req.createdAt)}
                    </p>
                    {/* 操作按钮 */}
                    <div className="flex gap-2 mt-2.5">
                      <button
                        onClick={() => handleRespond(req, true)}
                        disabled={handlingId === req.requestId}
                        className={cn(
                          "inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-lg transition-all",
                          "bg-green-500 text-white hover:bg-green-600 active:scale-95",
                          "shadow-sm shadow-green-200",
                          handlingId === req.requestId && "opacity-60 cursor-not-allowed"
                        )}
                      >
                        {handlingId === req.requestId ? (
                          <Loader2 className="w-3 h-3 animate-spin" />
                        ) : (
                          <Check className="w-3 h-3" />
                        )}
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
                        <X className="w-3 h-3" />
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )
        ) : outgoingReqs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400">
            <Clock className="w-12 h-12 mb-3 opacity-30" />
            <p className="text-sm">暂无发出的请求</p>
          </div>
        ) : (
          <div className="px-3 py-3 space-y-2">
            {outgoingReqs.map((req) => (
              <div
                key={req.requestId}
                className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80"
              >
                <UserAvatar
                  name={req.toUser?.displayName || "用户"}
                  size="md"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h4 className="text-sm font-medium text-gray-900">
                      {req.toUser?.displayName || "未知用户"}
                    </h4>
                    <span className="text-xs text-gray-400">
                      @{req.toUser?.username || ""}
                    </span>
                  </div>
                  {req.message && (
                    <p className="text-xs text-gray-500 mt-1 line-clamp-2">
                      {req.message}
                    </p>
                  )}
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs text-amber-600 bg-amber-50 rounded-full">
                      <Clock className="w-3 h-3" />
                      等待对方回应
                    </span>
                    <span className="text-xs text-gray-400">
                      {formatTimeAgo(req.createdAt)}
                    </span>
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
  const now = Date.now();
  const d = new Date(dateStr).getTime();
  if (isNaN(d)) return "";
  const diff = now - d;
  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`;
  return new Date(dateStr).toLocaleDateString("zh-CN");
}
