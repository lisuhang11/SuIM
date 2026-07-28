"use client";

import React, { useEffect, useState } from "react";
import { Check, CheckCheck, Clock, Download, FileText, Loader2, RotateCcw } from "lucide-react";
import type { Message } from "@/types";
import { cn, formatTime } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

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
    import("@/services/api").then(({ getFileDownloadURL }) => getFileDownloadURL(file.fileId)).then((value) => active && setUrl(value)).catch(() => undefined).finally(() => active && setLoading(false));
    return () => { active = false; };
  }, [file, message.type]);

  if (!file) return <>{message.content}</>;
  const download = async () => {
    setLoading(true);
    try {
      const { getFileDownloadURL } = await import("@/services/api");
      window.location.assign(await getFileDownloadURL(file.fileId));
    } finally { setLoading(false); }
  };

  if (message.type === "image") {
    return <div className="min-w-44">
      <div className="flex min-h-28 items-center justify-center overflow-hidden rounded bg-black/5">
        {url ? <img src={url} alt={file.name} className="max-h-80 w-auto max-w-full object-contain" /> : loading ? <Loader2 className="h-5 w-5 animate-spin opacity-60" /> : <FileText className="h-8 w-8 opacity-50" />}
      </div>
      <button onClick={download} className={cn("mt-2 flex w-full items-center justify-between gap-3 text-left text-xs", isMine ? "text-emerald-50" : "text-slate-500")}>
        <span className="min-w-0 truncate">{file.name}</span><Download className="h-3.5 w-3.5 flex-shrink-0" />
      </button>
    </div>;
  }

  return <button onClick={download} disabled={loading} className="flex min-w-56 max-w-72 items-center gap-3 text-left">
    <span className={cn("flex h-10 w-10 flex-shrink-0 items-center justify-center rounded", isMine ? "bg-white/15" : "bg-slate-100")}><FileText className="h-5 w-5" /></span>
    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium">{file.name}</span><span className={cn("block text-[11px]", isMine ? "text-emerald-100" : "text-slate-400")}>{formatBytes(file.size)}</span></span>
    {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4 flex-shrink-0" />}
  </button>;
}

export default function MessageBubble({ message, isMine, isGroup, showAvatar }: { message: Message; isMine: boolean; isGroup: boolean; showAvatar: boolean }) {
  if (message.type === "system") {
    return <div className="my-5 flex justify-center px-4"><span className="rounded bg-slate-200/70 px-2.5 py-1 text-[11px] text-slate-500">{message.content}</span></div>;
  }

  const status = message.status === "sending" ? <Clock className="h-3 w-3" /> : message.status === "read" ? <CheckCheck className="h-3.5 w-3.5 text-emerald-500" /> : message.status === "failed" ? <RotateCcw className="h-3.5 w-3.5 text-rose-500" /> : message.status === "delivered" ? <CheckCheck className="h-3.5 w-3.5" /> : <Check className="h-3.5 w-3.5" />;

  return <div className={cn("mb-4 flex gap-2.5 px-4 sm:px-7", isMine && "flex-row-reverse")}>
    {!isMine && isGroup ? showAvatar ? <UserAvatar src={message.senderAvatar} name={message.senderName} size="sm" className="mt-5" /> : <div className="h-8 w-8 flex-shrink-0" /> : null}
    <div className={cn("flex max-w-[78%] flex-col sm:max-w-[66%]", isMine ? "items-end" : "items-start")}>
      {!isMine && isGroup && showAvatar && <span className="mb-1 px-0.5 text-[11px] text-slate-400">{message.senderName}</span>}
      <div className={cn("rounded-lg px-3.5 py-2.5 text-[14px] leading-6 shadow-sm", isMine ? "bg-emerald-600 text-white" : "border border-slate-200 bg-white text-slate-800")}><Attachment message={message} isMine={isMine} /></div>
      <div className={cn("mt-1 flex items-center gap-1 text-[10px] text-slate-400", isMine && "flex-row-reverse")}><time>{formatTime(message.createdAt)}</time>{isMine && status}{isMine && message.status === "read" && <span className="text-emerald-600">已读</span>}</div>
    </div>
  </div>;
}
