"use client";

// ============================================================
// FriendRequestsPanel — 好友请求管理（收到的/发出的）
// ============================================================
import React, { useState, useEffect, useCallback } from "react";
import { UserCheck, Clock, Check, X, Loader2, RefreshCw } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";
import type { FriendRequest } from "@/types";

type TabType = "incoming" | "outgoing";

export default function FriendRequestsPanel() {
  const { refreshConversations, refreshContacts, refreshFriendRequestBadge, friendRequestVersion } = useChat();
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
      const [incomingRes, outgoingRes] = await Promise.allSettled([
        IMSDK.getFriendApplicationListAsRecipient({ handleResults: [0] }),
        IMSDK.getFriendApplicationListAsApplicant({ handleResults: [0] }),
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
        if (accept) {
          await IMSDK.acceptFriendApplication(request.fromUserId, "已同意");
        } else {
          await IMSDK.refuseFriendApplication(request.fromUserId, "");
        }

        // 从列表移除
        setIncomingReqs((prev) => prev.filter((r) => r.requestId !== request.requestId));
        setFeedback({
          type: "success",
          message: accept
            ? `已同意${request.fromUser?.displayName || "该用户"}的好友请求`
            : "已拒绝好友请求",
        });
        if (accept) {
          await Promise.all([refreshConversations(), refreshContacts()]);
        }
        refreshFriendRequestBadge();
      } catch {
        setFeedback({ type: "error", message: "处理失败，请稍后再试" });
      } finally {
        setHandlingId(null);
      }
    },
    [handlingId, refreshConversations, refreshContacts, refreshFriendRequestBadge]
  );

  return (
    <div className="h-full flex flex-col bg-surface-elevated w-full">
      {/* 头部 */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-edge flex-shrink-0">
        <div>
          <h2 className="text-sm font-semibold text-ink">好友请求</h2>
          <p className="text-[10px] text-ink-muted">
            {incomingReqs.length > 0
              ? `${incomingReqs.length} 条待处理`
              : "暂无待处理请求"}
          </p>
        </div>
        <button
          onClick={loadRequests}
          className={cn(
            "ui-press p-1.5 rounded-control hover:bg-surface-muted text-ink-muted hover:text-ink transition-colors",
            loading && "animate-spin"
          )}
          title="刷新"
        >
          <RefreshCw className="w-4 h-4" strokeWidth={1.75} />
        </button>
      </div>

      {/* 标签切换 */}
      <div className="flex border-b border-edge px-4">
        <button
          onClick={() => { setTab("incoming"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "incoming"
              ? "text-accent"
              : "text-ink-muted hover:text-ink"
          )}
        >
          收到的请求
          {incomingReqs.length > 0 && (
            <span className="ml-1.5 px-1.5 py-0.5 text-xs font-bold text-white bg-danger rounded-control">
              {incomingReqs.length}
            </span>
          )}
          {tab === "incoming" && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-accent rounded-full" />
          )}
        </button>
        <button
          onClick={() => { setTab("outgoing"); setFeedback(null); }}
          className={cn(
            "relative px-4 py-2.5 text-sm font-medium transition-colors",
            tab === "outgoing"
              ? "text-accent"
              : "text-ink-muted hover:text-ink"
          )}
        >
          发出的请求
          {tab === "outgoing" && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-accent rounded-full" />
          )}
        </button>
      </div>

      {/* 反馈 */}
      {feedback && (
        <div
          className={cn(
            "mx-4 mt-3 px-3 py-2 rounded-control text-sm",
            feedback.type === "success"
              ? "bg-accent-soft text-accent"
              : "bg-danger-soft text-danger"
          )}
        >
          {feedback.message}
        </div>
      )}

      {/* 列表 */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="w-5 h-5 text-accent animate-spin" strokeWidth={1.75} />
            <span className="ml-2 text-sm text-ink-muted">加载中...</span>
          </div>
        ) : tab === "incoming" ? (
          incomingReqs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-ink-muted">
              <UserCheck className="w-12 h-12 mb-3 opacity-30" strokeWidth={1.75} />
              <p className="text-sm">暂无收到的请求</p>
            </div>
          ) : (
            <div className="px-3 py-3 space-y-2">
              {incomingReqs.map((req) => (
                <div
                  key={req.requestId}
                  className="flex items-start gap-3 p-3 rounded-control bg-surface-muted/80"
                >
                  <UserAvatar
                    name={req.fromUser?.displayName || "用户"}
                    size="md"
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-medium text-ink">
                        {req.fromUser?.displayName || "未知用户"}
                      </h4>
                      <span className="text-xs text-ink-muted">
                        @{req.fromUser?.username || ""}
                      </span>
                    </div>
                    {req.message && (
                      <p className="text-xs text-ink-muted mt-1 line-clamp-2">
                        {req.message}
                      </p>
                    )}
                    <p className="text-xs text-ink-muted mt-1.5">
                      {formatTimeAgo(req.createdAt)}
                    </p>
                    {/* 操作按钮 */}
                    <div className="flex gap-2 mt-2.5">
                      <button
                        onClick={() => handleRespond(req, true)}
                        disabled={handlingId === req.requestId}
                        className={cn(
                          "ui-press inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-control transition-all",
                          "bg-accent text-accent-fg hover:bg-accent-hover active:scale-95",
                          handlingId === req.requestId && "opacity-60 cursor-not-allowed"
                        )}
                      >
                        {handlingId === req.requestId ? (
                          <Loader2 className="w-3 h-3 animate-spin" strokeWidth={1.75} />
                        ) : (
                          <Check className="w-3 h-3" strokeWidth={1.75} />
                        )}
                        同意
                      </button>
                      <button
                        onClick={() => handleRespond(req, false)}
                        disabled={handlingId === req.requestId}
                        className={cn(
                          "ui-press inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium rounded-control transition-all",
                          "bg-surface-elevated text-ink-muted border border-edge hover:bg-surface-muted active:scale-95",
                          handlingId === req.requestId && "opacity-60 cursor-not-allowed"
                        )}
                      >
                        <X className="w-3 h-3" strokeWidth={1.75} />
                        拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )
        ) : outgoingReqs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-ink-muted">
            <Clock className="w-12 h-12 mb-3 opacity-30" strokeWidth={1.75} />
            <p className="text-sm">暂无发出的请求</p>
          </div>
        ) : (
          <div className="px-3 py-3 space-y-2">
            {outgoingReqs.map((req) => (
              <div
                key={req.requestId}
                className="flex items-start gap-3 p-3 rounded-control bg-surface-muted/80"
              >
                <UserAvatar
                  name={req.toUser?.displayName || "用户"}
                  size="md"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h4 className="text-sm font-medium text-ink">
                      {req.toUser?.displayName || "未知用户"}
                    </h4>
                    <span className="text-xs text-ink-muted">
                      @{req.toUser?.username || ""}
                    </span>
                  </div>
                  {req.message && (
                    <p className="text-xs text-ink-muted mt-1 line-clamp-2">
                      {req.message}
                    </p>
                  )}
                  <div className="flex items-center gap-2 mt-1.5">
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs text-amber-600 bg-amber-50 rounded-control">
                      <Clock className="w-3 h-3" strokeWidth={1.75} />
                      等待对方回应
                    </span>
                    <span className="text-xs text-ink-muted">
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
