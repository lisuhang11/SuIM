"use client";

import React from "react";
import { LockKeyhole, MessageCircle } from "lucide-react";

export default function EmptyChat() {
  return <div className="flex flex-1 flex-col items-center justify-center bg-surface-muted px-6 text-center"><div className="flex h-14 w-14 items-center justify-center rounded-control border border-edge bg-surface-elevated text-accent shadow-sm"><MessageCircle className="h-6 w-6" strokeWidth={1.75} /></div><h2 className="mt-4 text-base font-semibold text-ink">选择一个会话</h2><p className="mt-1 max-w-xs text-sm leading-6 text-ink-muted">从左侧打开最近联系人或群组，继续你的对话。</p><p className="mt-7 flex items-center gap-1.5 text-[11px] text-ink-muted"><LockKeyhole className="h-3 w-3" strokeWidth={1.75} />消息传输由 SuIM 服务保护</p></div>;
}
