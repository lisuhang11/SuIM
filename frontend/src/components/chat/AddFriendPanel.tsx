"use client";

// ============================================================
// AddFriendPanel — 通过用户 ID 精确搜索并发送好友请求
// ============================================================
import React, { useState, useCallback } from "react";
import { Search, UserPlus, Clock, Loader2, Check } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";
import type { User, SearchedUser } from "@/types";

const MAX_REQUEST_MSG = 512;

interface AddFriendPanelProps {
  embedded?: boolean;
}

function friendRequestErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err ?? "");
  const msg = raw.trim().toLowerCase();
  if (msg.includes("already friends") || msg.includes("已是好友")) return "你们已经是好友了";
  if (msg.includes("already sent") || msg.includes("already requested")) return "已发送过好友申请，请等待对方处理";
  if (msg.includes("blocked by this user") || msg.includes("you are blocked")) return "对方已将你拉黑，无法发送申请";
  if (msg.includes("cannot friend yourself") || msg.includes("不能添加自己")) return "不能添加自己为好友";
  if (msg.includes("not found") || msg.includes("不存在")) return "用户不存在";
  return raw || "发送失败，请稍后再试";
}

function defaultRequestMessage(displayName?: string): string {
  const name = displayName?.trim();
  return name ? `我是${name}` : "";
}

export default function AddFriendPanel({ embedded = false }: AddFriendPanelProps) {
  const { user: currentUser } = useAuth();
  const { contacts } = useChat();
  const [searchQuery, setSearchQuery] = useState("");
  const [results, setResults] = useState<SearchedUser[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const [sendingId, setSendingId] = useState<string | null>(null);
  const [sentSet, setSentSet] = useState<Set<string>>(new Set());
  const [composingUserId, setComposingUserId] = useState<string | null>(null);
  const [requestMessage, setRequestMessage] = useState("");
  const [feedback, setFeedback] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const resetCompose = useCallback(() => {
    setComposingUserId(null);
    setRequestMessage("");
  }, []);

  const openCompose = useCallback(
    (userId: string) => {
      setFeedback(null);
      setComposingUserId(userId);
      setRequestMessage(defaultRequestMessage(currentUser?.displayName));
    },
    [currentUser?.displayName]
  );

  const runSearch = useCallback(async () => {
    const query = searchQuery.trim();
    setFeedback(null);
    resetCompose();
    if (!query) {
      setResults([]);
      setHasSearched(false);
      return;
    }
    if (query === currentUser?.userId) {
      setResults([]);
      setHasSearched(true);
      setFeedback({ type: "error", message: "不能添加自己为好友" });
      return;
    }

    setIsSearching(true);
    setHasSearched(true);
    try {
      const users = await IMSDK.searchUsers(query);
      const friendIds = new Set(contacts.map((c) => c.userId));
      const enriched: SearchedUser[] = users
        .filter((u) => u.userId !== currentUser?.userId)
        .map((u) => ({
          ...u,
          isFriend: friendIds.has(u.userId),
          hasSentRequest: sentSet.has(u.userId),
          hasIncomingRequest: false,
        }));
      setResults(enriched);
      if (enriched.length === 0) {
        setFeedback({ type: "error", message: "未找到该用户 ID" });
      }
    } catch (err) {
      setFeedback({
        type: "error",
        message: err instanceof Error && err.message ? err.message : "搜索失败，请稍后再试",
      });
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  }, [searchQuery, currentUser?.userId, sentSet, contacts, resetCompose]);

  const handleSendRequest = useCallback(async (targetUser: User & { isFriend?: boolean }) => {
    if (sendingId || !currentUser) return;
    if (targetUser.isFriend) {
      setFeedback({ type: "error", message: "你们已经是好友了" });
      return;
    }
    const message = requestMessage.trim().slice(0, MAX_REQUEST_MSG);
    setSendingId(targetUser.userId);
    setFeedback(null);

    try {
      await IMSDK.addFriend(targetUser.userId, message);
      setSentSet((prev) => new Set(prev).add(targetUser.userId));
      setFeedback({ type: "success", message: `已向 ${targetUser.displayName} 发送好友请求` });
      setResults((prev) =>
        prev.map((u) =>
          u.userId === targetUser.userId ? { ...u, hasSentRequest: true } : u
        )
      );
      resetCompose();
    } catch (err) {
      setFeedback({ type: "error", message: friendRequestErrorMessage(err) });
    } finally {
      setSendingId(null);
    }
  }, [sendingId, currentUser, requestMessage, resetCompose]);

  return (
    <div className="flex h-full flex-col bg-surface-elevated">
      {!embedded && (
        <div className="border-b border-edge px-4 py-4">
          <h3 className="text-base font-semibold text-ink">添加好友</h3>
          <p className="mt-0.5 text-xs text-ink-muted">仅支持通过用户 ID 精确查找</p>
        </div>
      )}

      <div className="px-4 py-3">
        {embedded && (
          <p className="mb-2 text-xs text-ink-muted">仅支持用户 ID 精确查找</p>
        )}
        <div className="flex gap-2">
          <div className="relative min-w-0 flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-muted" strokeWidth={1.75} />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value);
                setHasSearched(false);
                setResults([]);
                setFeedback(null);
                resetCompose();
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter") void runSearch();
              }}
              placeholder="输入完整用户 ID"
              spellCheck={false}
              autoCorrect="off"
              autoCapitalize="off"
              className="h-10 w-full rounded-control border border-edge bg-surface-muted py-0 pl-9 pr-3 text-sm outline-none transition focus:border-accent focus:bg-surface-elevated focus:ring-2 focus:ring-accent/20"
            />
          </div>
          <button
            type="button"
            onClick={() => void runSearch()}
            disabled={isSearching || !searchQuery.trim()}
            className="ui-press flex h-10 flex-none items-center gap-1.5 rounded-control bg-rail px-3 text-sm font-medium text-surface-elevated hover:bg-ink disabled:opacity-50"
          >
            {isSearching ? <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} /> : <Search className="h-4 w-4" strokeWidth={1.75} />}
            查找
          </button>
        </div>
      </div>

      {feedback && (
        <div
          className={cn(
            "mx-4 mb-2 rounded-control px-3 py-2 text-sm",
            feedback.type === "success" ? "bg-accent-soft text-accent" : "bg-danger-soft text-danger"
          )}
        >
          {feedback.message}
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isSearching ? (
          <div className="flex items-center justify-center py-20">
            <Loader2 className="h-5 w-5 animate-spin text-accent" strokeWidth={1.75} />
            <span className="ml-2 text-sm text-ink-muted">查找中...</span>
          </div>
        ) : !hasSearched ? (
          <div className="flex flex-col items-center justify-center py-20 text-ink-muted">
            <Search className="mb-3 h-12 w-12 opacity-30" strokeWidth={1.75} />
            <p className="text-sm">输入对方的用户 ID 后点击查找</p>
            <p className="mt-1 text-xs opacity-60">不支持昵称或邮箱搜索</p>
          </div>
        ) : results.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-ink-muted">
            <UserPlus className="mb-3 h-12 w-12 opacity-30" strokeWidth={1.75} />
            <p className="text-sm">未找到该用户 ID</p>
            <p className="mt-1 text-xs opacity-60">请确认 ID 完整且无误</p>
          </div>
        ) : (
          <div className="space-y-2 px-3 pb-3">
            {results.map((user) => {
              const isComposing = composingUserId === user.userId;
              return (
                <div
                  key={user.userId}
                  className={cn(
                    "rounded-control px-3 py-2.5",
                    isComposing ? "bg-surface-muted ring-1 ring-edge" : "hover:bg-surface-muted"
                  )}
                >
                  <div className="flex items-center gap-3">
                    <UserAvatar name={user.displayName} size="md" />
                    <div className="min-w-0 flex-1">
                      <h4 className="truncate text-sm font-medium text-ink">
                        {user.displayName}
                      </h4>
                      <p className="truncate text-xs text-ink-muted">ID {user.userId}</p>
                    </div>
                    {user.isFriend ? (
                      <span className="inline-flex cursor-default items-center gap-1 rounded-control bg-accent-soft px-3 py-1.5 text-xs text-accent">
                        <Check className="h-3 w-3" strokeWidth={1.75} />
                        已是好友
                      </span>
                    ) : user.hasSentRequest ? (
                      <span className="inline-flex cursor-default items-center gap-1 rounded-control bg-surface-muted px-3 py-1.5 text-xs text-ink-muted">
                        <Clock className="h-3 w-3" strokeWidth={1.75} />
                        已申请
                      </span>
                    ) : !isComposing ? (
                      <button
                        type="button"
                        onClick={() => openCompose(user.userId)}
                        className="ui-press inline-flex items-center gap-1 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accent-fg hover:bg-accent-hover"
                      >
                        <UserPlus className="h-3 w-3" strokeWidth={1.75} />
                        添加
                      </button>
                    ) : null}
                  </div>

                  {isComposing && !user.isFriend && !user.hasSentRequest && (
                    <div className="mt-3 space-y-2">
                      <label className="block text-xs font-medium text-ink-muted">
                        申请信息
                        <span className="ml-1 font-normal text-ink-muted">（选填）</span>
                      </label>
                      <textarea
                        value={requestMessage}
                        onChange={(e) => setRequestMessage(e.target.value.slice(0, MAX_REQUEST_MSG))}
                        rows={3}
                        maxLength={MAX_REQUEST_MSG}
                        placeholder="向对方打个招呼吧"
                        className="w-full resize-none rounded-control border border-edge bg-surface-elevated px-3 py-2 text-sm outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/20"
                      />
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-[11px] text-ink-muted">
                          {requestMessage.length}/{MAX_REQUEST_MSG}
                        </span>
                        <div className="flex gap-2">
                          <button
                            type="button"
                            onClick={resetCompose}
                            disabled={sendingId === user.userId}
                            className="ui-press rounded-control border border-edge bg-surface-elevated px-3 py-1.5 text-xs font-medium text-ink-muted hover:bg-surface-muted disabled:opacity-50"
                          >
                            取消
                          </button>
                          <button
                            type="button"
                            onClick={() => void handleSendRequest(user)}
                            disabled={sendingId === user.userId}
                            className={cn(
                              "ui-press inline-flex items-center gap-1 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accent-fg hover:bg-accent-hover",
                              sendingId === user.userId && "cursor-not-allowed opacity-60"
                            )}
                          >
                            {sendingId === user.userId ? (
                              <Loader2 className="h-3 w-3 animate-spin" strokeWidth={1.75} />
                            ) : (
                              <UserPlus className="h-3 w-3" strokeWidth={1.75} />
                            )}
                            发送申请
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
