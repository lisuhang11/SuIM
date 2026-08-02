"use client";

import React, { useEffect, useState } from "react";
import { AlertCircle, Check, CheckCheck, Clock, Download, FileText, Loader2, Phone } from "lucide-react";
import type { Message } from "@/types";
import { cn, formatTime } from "@/lib/utils";
import { IMSDK } from "@/suim-sdk";
import { useChat } from "@/contexts/ChatContext";
import UserAvatar from "../shared/UserAvatar";

/** 与后端 repository.RevokeTimeLimit 一致：发送后 2 分钟内可撤回 */
const REVOKE_WINDOW_MS = 2 * 60 * 1000;

function revokeRemainingMs(createdAt: string): number {
  const sent = new Date(createdAt).getTime();
  if (!Number.isFinite(sent) || sent <= 0) return 0;
  return sent + REVOKE_WINDOW_MS - Date.now();
}

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function Attachment({ message, isMine }: { message: Message; isMine: boolean }) {
  const [url, setUrl] = useState("");
  const [loading, setLoading] = useState(message.type === "image");
  const file = message.file;

  useEffect(() => {
    if (!file || message.type !== "image") return;
    let active = true;
    IMSDK.getFileDownloadURL(file.fileId)
      .then((value) => active && setUrl(value))
      .catch(() => undefined)
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [file, message.type]);

  if (!file) return <>{message.content}</>;
  const download = async () => {
    setLoading(true);
    try {
      window.location.assign(await IMSDK.getFileDownloadURL(file.fileId));
    } finally {
      setLoading(false);
    }
  };

  if (message.type === "image") {
    return (
      <div className="min-w-44">
        <div className="flex min-h-28 items-center justify-center overflow-hidden rounded-control bg-black/5 dark:bg-white/5">
          {url ? (
            <img src={url} alt={file.name} className="max-h-80 w-auto max-w-full object-contain" />
          ) : loading ? (
            <Loader2 className="h-5 w-5 animate-spin opacity-60" />
          ) : (
            <FileText className="h-8 w-8 opacity-50" />
          )}
        </div>
        <button
          onClick={download}
          className={cn(
            "mt-2 flex w-full items-center justify-between gap-3 text-left text-xs",
            isMine ? "text-accent-fg/80" : "text-ink-muted"
          )}
        >
          <span className="min-w-0 truncate">{file.name}</span>
          <Download className="h-3.5 w-3.5 flex-shrink-0" strokeWidth={1.75} />
        </button>
      </div>
    );
  }

  return (
    <button onClick={download} disabled={loading} className="flex min-w-56 max-w-72 items-center gap-3 text-left">
      <span
        className={cn(
          "flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-control",
          isMine ? "bg-black/10 dark:bg-white/15" : "bg-surface-muted"
        )}
      >
        <FileText className="h-5 w-5" strokeWidth={1.75} />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium">{file.name}</span>
        <span className={cn("block text-[11px]", isMine ? "text-accent-fg/70" : "text-ink-muted")}>
          {formatBytes(file.size)}
        </span>
      </span>
      {loading ? (
        <Loader2 className="h-4 w-4 animate-spin" />
      ) : (
        <Download className="h-4 w-4 flex-shrink-0" strokeWidth={1.75} />
      )}
    </button>
  );
}

export default function MessageBubble({
  message,
  isMine,
  isGroup,
  showAvatar = true,
  showName = true,
  avatarSrc,
  displayName,
}: {
  message: Message;
  isMine: boolean;
  isGroup: boolean;
  showAvatar?: boolean;
  showName?: boolean;
  avatarSrc?: string;
  displayName?: string;
}) {
  const { revokeMessage } = useChat();
  const [revoking, setRevoking] = useState(false);
  const [withinRevokeWindow, setWithinRevokeWindow] = useState(
    () => revokeRemainingMs(message.createdAt) > 0
  );

  useEffect(() => {
    const remaining = revokeRemainingMs(message.createdAt);
    if (remaining <= 0) {
      setWithinRevokeWindow(false);
      return;
    }
    setWithinRevokeWindow(true);
    const timer = window.setTimeout(() => setWithinRevokeWindow(false), remaining);
    return () => window.clearTimeout(timer);
  }, [message.createdAt]);

  if (message.type === "system" || message.status === "revoked") {
    const isCallRecord =
      message.content.startsWith("语音通话") ||
      ["已拒绝", "已取消", "未接来电", "忙线未接通", "对方不在线"].includes(message.content);
    return (
      <div className="my-5 flex justify-center px-4">
        <span className="inline-flex items-center gap-1.5 rounded-control bg-surface-muted px-2.5 py-1 text-[11px] text-ink-muted">
          {isCallRecord ? <Phone className="h-3 w-3 flex-none" strokeWidth={1.75} /> : null}
          {message.content || "撤回了一条消息"}
        </span>
      </div>
    );
  }

  const name = displayName || message.senderName || message.senderId || "?";
  const avatar = avatarSrc || message.senderAvatar || "";
  const failed = message.status === "failed";
  const canRevoke =
    isMine &&
    withinRevokeWindow &&
    !!message.clientMsgId &&
    (message.status === "sent" ||
      message.status === "delivered" ||
      message.status === "read");
  const status =
    message.status === "sending" ? (
      <Clock className="h-3 w-3" strokeWidth={1.75} />
    ) : message.status === "read" ? (
      <CheckCheck className="h-3.5 w-3.5 text-accent" strokeWidth={1.75} />
    ) : failed ? null : message.status === "delivered" ? (
      <CheckCheck className="h-3.5 w-3.5" strokeWidth={1.75} />
    ) : (
      <Check className="h-3.5 w-3.5" strokeWidth={1.75} />
    );

  const onRevoke = async () => {
    if (!canRevoke || revoking || !message.clientMsgId) return;
    setRevoking(true);
    try {
      await revokeMessage(message.conversationId, message.clientMsgId);
    } catch {
      // keep bubble; server/network error
    } finally {
      setRevoking(false);
    }
  };

  return (
    <div className={cn("mb-4 flex gap-2.5 px-4 sm:px-7", isMine && "flex-row-reverse")}>
      {showAvatar ? (
        <UserAvatar
          src={avatar}
          name={name}
          size="sm"
          className={cn("mt-0.5 rounded-control", isGroup && !isMine && showName && "mt-5")}
        />
      ) : (
        <div className="h-8 w-8 flex-shrink-0" />
      )}
      <div className={cn("flex max-w-[78%] flex-col sm:max-w-[66%]", isMine ? "items-end" : "items-start")}>
        {!isMine && isGroup && showName && (
          <span className="mb-1 px-0.5 text-[11px] text-ink-muted">{name}</span>
        )}
        <div className="flex items-center gap-1.5">
          {failed && isMine ? (
            <span className="flex-none text-danger" title="发送失败" aria-label="发送失败">
              <AlertCircle className="h-5 w-5" strokeWidth={2} />
            </span>
          ) : null}
          <div
            className={cn(
              "group/bubble relative px-3.5 py-2.5 text-[14px] leading-6",
              isMine
                ? "rounded-bubble rounded-br-[4px] bg-accent text-accent-fg"
                : "rounded-bubble rounded-bl-[4px] border border-edge bg-surface-elevated text-ink"
            )}
          >
            <Attachment message={message} isMine={isMine} />
            {canRevoke ? (
              <button
                type="button"
                onClick={onRevoke}
                disabled={revoking}
                className={cn(
                  "absolute -top-2 text-[10px] opacity-0 transition-opacity group-hover/bubble:opacity-100",
                  isMine ? "-left-12 text-ink-muted" : "-right-12 text-ink-muted"
                )}
                title="撤回"
              >
                {revoking ? "…" : "撤回"}
              </button>
            ) : null}
          </div>
        </div>
        <div
          className={cn(
            "mt-1 flex items-center gap-1 text-[10px] text-ink-muted",
            isMine && "flex-row-reverse"
          )}
        >
          <time>{formatTime(message.createdAt)}</time>
          {isMine && status}
          {isMine && message.status === "read" && <span className="text-accent">已读</span>}
        </div>
      </div>
    </div>
  );
}
