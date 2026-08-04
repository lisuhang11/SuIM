"use client";

import React, { useCallback, useEffect, useState } from "react";
import { Ban, Loader2, RefreshCw } from "lucide-react";
import { IMSDK } from "@/suim-sdk";
import { useChat } from "@/contexts/ChatContext";
import type { BlacklistEntry } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 50;

function formatBlockTime(ms: number): string {
  if (!ms || ms <= 0) return "";
  try {
    return new Date(ms).toLocaleString("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return "";
  }
}

export default function BlacklistPanel() {
  const { refreshContacts } = useChat();
  const [list, setList] = useState<BlacklistEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [feedback, setFeedback] = useState("");

  const load = useCallback(async (mode: "replace" | "append" = "replace") => {
    if (mode === "replace") {
      setLoading(true);
      setError("");
      setFeedback("");
    } else {
      setLoadingMore(true);
    }
    try {
      const offset =
        mode === "append"
          ? await new Promise<number>((resolve) => {
              setList((prev) => {
                resolve(prev.length);
                return prev;
              });
            })
          : 0;
      const page = await IMSDK.getBlackList({ offset, limit: PAGE_SIZE });
      setList((prev) => (mode === "append" ? [...prev, ...page] : page));
      setHasMore(page.length >= PAGE_SIZE);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载黑名单失败");
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    void load("replace");
  }, [load]);

  const handleUnblock = async (entry: BlacklistEntry) => {
    if (busyId) return;
    if (!window.confirm(`将 ${entry.displayName || entry.username} 移出黑名单？`)) return;
    setBusyId(entry.userId);
    setFeedback("");
    setError("");
    try {
      await IMSDK.removeBlack(entry.userId);
      setList((prev) => prev.filter((item) => item.userId !== entry.userId));
      setFeedback(`已将 ${entry.displayName || entry.username} 移出黑名单`);
      await refreshContacts();
    } catch (e) {
      setError(e instanceof Error ? e.message : "取消拉黑失败");
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="flex h-full w-full flex-col bg-surface-elevated">
      <div className="flex h-14 flex-none items-center justify-between border-b border-edge px-4">
        <div>
          <h2 className="text-sm font-semibold text-ink">黑名单</h2>
          <p className="text-[10px] text-ink-muted">
            {list.length > 0 ? `${list.length} 人` : "管理已拉黑的用户"}
          </p>
        </div>
        <button
          type="button"
          onClick={() => void load("replace")}
          className={cn(
            "ui-press rounded-control p-1.5 text-ink-muted hover:bg-surface-muted hover:text-ink",
            loading && "animate-spin"
          )}
          title="刷新"
          disabled={loading}
        >
          <RefreshCw className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>

      {feedback ? (
        <p className="mx-4 mt-3 rounded-control bg-accent-soft px-3 py-2 text-xs text-accent">{feedback}</p>
      ) : null}
      {error ? (
        <div className="mx-4 mt-3 flex items-center justify-between gap-2 rounded-control bg-danger-soft px-3 py-2 text-xs text-danger">
          <span>{error}</span>
          <button type="button" className="underline" onClick={() => void load("replace")}>
            重试
          </button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex h-40 items-center justify-center text-ink-muted">
            <Loader2 className="h-5 w-5 animate-spin" strokeWidth={1.75} />
          </div>
        ) : list.length === 0 ? (
          <div className="flex h-48 flex-col items-center justify-center gap-2 text-ink-muted">
            <Ban className="h-8 w-8 opacity-40" strokeWidth={1.75} />
            <p className="text-sm">暂无黑名单用户</p>
          </div>
        ) : (
          <ul>
            {list.map((entry) => {
              const title = entry.displayName || entry.username || entry.userId;
              const timeLabel = formatBlockTime(entry.createTime);
              const busy = busyId === entry.userId;
              return (
                <li
                  key={entry.userId}
                  className="flex items-center gap-3 border-b border-edge px-4 py-3"
                >
                  <UserAvatar src={entry.avatar} name={title} size="md" />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-ink">{title}</p>
                    <p className="truncate text-xs text-ink-muted">
                      @{entry.username || entry.userId}
                      {timeLabel ? ` · ${timeLabel}` : null}
                    </p>
                  </div>
                  <button
                    type="button"
                    disabled={!!busyId}
                    onClick={() => void handleUnblock(entry)}
                    className="ui-press flex h-8 items-center gap-1 rounded-control border border-edge px-2.5 text-xs text-ink-muted hover:bg-surface-muted disabled:opacity-50"
                  >
                    {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" strokeWidth={1.75} /> : null}
                    取消拉黑
                  </button>
                </li>
              );
            })}
          </ul>
        )}

        {hasMore && !loading ? (
          <div className="flex justify-center py-4">
            <button
              type="button"
              disabled={loadingMore}
              onClick={() => void load("append")}
              className="ui-press rounded-control border border-edge px-4 py-1.5 text-xs text-ink-muted hover:bg-surface-muted disabled:opacity-50"
            >
              {loadingMore ? "加载中…" : "加载更多"}
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
