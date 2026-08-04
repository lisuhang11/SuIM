"use client";

import React, { useMemo, useState } from "react";
import { Search, SlidersHorizontal } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import ConversationItem from "./ConversationItem";

interface ConversationListProps {
  onOpenConversation?: () => void;
}

export default function ConversationList({ onOpenConversation }: ConversationListProps) {
  const { user } = useAuth();
  const { conversations, activeConversationId, setActiveConversation, wsConnected } = useChat();
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "unread" | "groups">("all");

  const sorted = useMemo(() => conversations
    .filter((item) => {
      const matchesSearch = item.title.toLowerCase().includes(search.trim().toLowerCase());
      const matchesFilter = filter === "all" || (filter === "unread" && item.unreadCount > 0) || (filter === "groups" && item.type === "group");
      return matchesSearch && matchesFilter;
    })
    .sort((a, b) => Number(b.isPinned) - Number(a.isPinned) || new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()), [conversations, filter, search]);

  const open = (id: string) => {
    setActiveConversation(id);
    onOpenConversation?.();
  };

  return (
    <div className="flex h-full w-full flex-col bg-surface-elevated">
      <header className="px-5 pb-3 pt-5">
        <div>
          <p className="text-xs font-medium text-ink-muted">SUIM WORKSPACE</p>
          <h1 className="mt-1 text-xl font-semibold text-ink">消息</h1>
        </div>
        <div className="mt-4 flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-muted" strokeWidth={1.75} />
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索联系人或会话" className="h-10 w-full rounded-control border border-edge bg-surface-muted pl-9 pr-3 text-sm text-ink outline-none transition focus:border-accent focus:bg-surface-elevated" />
          </div>
          <button className="ui-press flex h-10 w-10 items-center justify-center rounded-control border border-edge text-ink-muted" title="筛选设置"><SlidersHorizontal className="h-4 w-4" strokeWidth={1.75} /></button>
        </div>
      </header>

      <div className="flex items-center gap-1 border-b border-edge px-4 pb-3">
        {([['all', '全部'], ['unread', '未读'], ['groups', '群聊']] as const).map(([id, label]) => (
          <button key={id} onClick={() => setFilter(id)} className={cn("ui-press h-8 rounded-control px-3 text-xs font-medium", filter === id ? "bg-rail text-surface-elevated" : "text-ink-muted hover:bg-surface-muted")}>{label}</button>
        ))}
        <span className="ml-auto flex items-center gap-1.5 text-[11px] text-ink-muted"><span className={cn("h-1.5 w-1.5 rounded-full", wsConnected ? "bg-accent" : "bg-danger")} />{wsConnected ? "实时连接" : "连接中断"}</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto py-2">
        {sorted.map((conversation) => <ConversationItem key={conversation.conversationId} conversation={conversation} isActive={conversation.conversationId === activeConversationId} onClick={() => open(conversation.conversationId)} />)}
        {sorted.length === 0 && <div className="px-8 py-16 text-center"><p className="text-sm font-medium text-ink-muted">没有匹配的会话</p><p className="mt-1 text-xs text-ink-muted">换个关键词或筛选条件试试</p></div>}
      </div>

      <footer className="flex items-center justify-between border-t border-edge px-5 py-3 text-xs text-ink-muted">
        <span>{user?.displayName}</span><span>{conversations.reduce((sum, item) => sum + item.unreadCount, 0)} 条未读</span>
      </footer>
    </div>
  );
}
