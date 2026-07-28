"use client";

import React, { useMemo, useState } from "react";
import { Edit3, Search, SlidersHorizontal } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import ConversationItem from "./ConversationItem";
import CreateGroupDialog from "./CreateGroupDialog";

interface ConversationListProps {
  onOpenConversation?: () => void;
}

export default function ConversationList({ onOpenConversation }: ConversationListProps) {
  const { user } = useAuth();
  const { conversations, activeConversationId, setActiveConversation, wsConnected } = useChat();
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<"all" | "unread" | "groups">("all");
  const [showCreateGroup, setShowCreateGroup] = useState(false);

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
    <div className="flex h-full w-full flex-col bg-white">
      <header className="px-5 pb-3 pt-5">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xs font-medium text-slate-400">SUIM WORKSPACE</p>
            <h1 className="mt-1 text-xl font-semibold text-slate-900">消息</h1>
          </div>
          <button onClick={() => setShowCreateGroup(true)} className="flex h-9 w-9 items-center justify-center rounded-md border border-slate-200 text-slate-600 shadow-sm hover:border-emerald-300 hover:text-emerald-600" title="发起会话">
            <Edit3 className="h-4 w-4" />
          </button>
        </div>
        <div className="mt-4 flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索联系人或会话" className="h-10 w-full rounded-md border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm text-slate-800 outline-none transition focus:border-emerald-400 focus:bg-white" />
          </div>
          <button className="flex h-10 w-10 items-center justify-center rounded-md border border-slate-200 text-slate-500" title="筛选设置"><SlidersHorizontal className="h-4 w-4" /></button>
        </div>
      </header>

      <div className="flex items-center gap-1 border-b border-slate-100 px-4 pb-3">
        {([['all', '全部'], ['unread', '未读'], ['groups', '群聊']] as const).map(([id, label]) => (
          <button key={id} onClick={() => setFilter(id)} className={cn("h-8 rounded-md px-3 text-xs font-medium", filter === id ? "bg-slate-900 text-white" : "text-slate-500 hover:bg-slate-100")}>{label}</button>
        ))}
        <span className="ml-auto flex items-center gap-1.5 text-[11px] text-slate-400"><span className={cn("h-1.5 w-1.5 rounded-full", wsConnected ? "bg-emerald-500" : "bg-rose-500")} />{wsConnected ? "实时连接" : "连接中断"}</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto py-2">
        {sorted.map((conversation) => <ConversationItem key={conversation.conversationId} conversation={conversation} isActive={conversation.conversationId === activeConversationId} onClick={() => open(conversation.conversationId)} />)}
        {sorted.length === 0 && <div className="px-8 py-16 text-center"><p className="text-sm font-medium text-slate-600">没有匹配的会话</p><p className="mt-1 text-xs text-slate-400">换个关键词或筛选条件试试</p></div>}
      </div>

      <footer className="flex items-center justify-between border-t border-slate-100 px-5 py-3 text-xs text-slate-400">
        <span>{user?.displayName}</span><span>{conversations.reduce((sum, item) => sum + item.unreadCount, 0)} 条未读</span>
      </footer>
      {showCreateGroup && <CreateGroupDialog onClose={() => setShowCreateGroup(false)} />}
    </div>
  );
}
