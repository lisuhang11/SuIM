"use client";

import React, { useMemo, useState } from "react";
import { Check, MessageCircle, Search, Send, UserPlus, X } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { mockFriendRequests } from "@/services/mock-data";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

interface FriendsPanelProps { onOpenConversation?: () => void }

export default function FriendsPanel({ onOpenConversation }: FriendsPanelProps) {
  const { contacts, conversations, setActiveConversation } = useChat();
  const [tab, setTab] = useState<"contacts" | "requests">("contacts");
  const [query, setQuery] = useState("");
  const [handled, setHandled] = useState<Record<string, "accepted" | "rejected">>({});
  const [showAddFriend, setShowAddFriend] = useState(false);
  const [requestSent, setRequestSent] = useState(false);
  const filtered = useMemo(() => contacts.filter((item) => `${item.displayName}${item.username}`.toLowerCase().includes(query.toLowerCase())), [contacts, query]);

  const openChat = (userId: string) => {
    const conversation = conversations.find((item) => item.type === "private" && item.members.some((member) => member.userId === userId));
    if (conversation) {
      setActiveConversation(conversation.conversationId);
      onOpenConversation?.();
    }
  };

  return (
    <div className="flex h-full w-full flex-col bg-white">
      <header className="px-5 pb-3 pt-5">
        <div className="flex items-center justify-between"><div><p className="text-xs font-medium text-slate-400">RELATIONS</p><h1 className="mt-1 text-xl font-semibold text-slate-900">通讯录</h1></div><button onClick={() => { setShowAddFriend(true); setRequestSent(false); }} className="flex h-9 w-9 items-center justify-center rounded-md border border-slate-200 text-slate-600" title="添加好友"><UserPlus className="h-4 w-4" /></button></div>
        <div className="mt-4 flex rounded-md bg-slate-100 p-1">
          <button onClick={() => setTab("contacts")} className={cn("h-8 flex-1 rounded text-xs font-medium", tab === "contacts" ? "bg-white text-slate-900 shadow-sm" : "text-slate-500")}>好友 {contacts.length}</button>
          <button onClick={() => setTab("requests")} className={cn("h-8 flex-1 rounded text-xs font-medium", tab === "requests" ? "bg-white text-slate-900 shadow-sm" : "text-slate-500")}>新的朋友 <span className="ml-1 rounded-full bg-rose-500 px-1.5 text-[10px] text-white">2</span></button>
        </div>
      </header>

      {tab === "contacts" ? <>
        <div className="px-5 pb-3"><div className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索好友" className="h-10 w-full rounded-md border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm outline-none focus:border-emerald-400" /></div></div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          <p className="px-5 py-2 text-[11px] font-semibold uppercase text-slate-400">联系人</p>
          {filtered.map((contact) => <div key={contact.userId} className="group flex items-center gap-3 px-5 py-3 hover:bg-slate-50"><div className="relative"><UserAvatar src={contact.avatar} name={contact.displayName} size="md" /><span className={cn("absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-white", contact.status === "online" ? "bg-emerald-500" : contact.status === "away" ? "bg-amber-400" : "bg-slate-300")} /></div><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium text-slate-800">{contact.displayName}</p><p className="truncate text-xs text-slate-400">@{contact.username} · {contact.status === "online" ? "在线" : contact.status === "away" ? "离开" : "离线"}</p></div><button onClick={() => openChat(contact.userId)} className="flex h-8 w-8 items-center justify-center rounded-md text-slate-400 opacity-0 hover:bg-emerald-50 hover:text-emerald-600 group-hover:opacity-100" title="发消息"><MessageCircle className="h-4 w-4" /></button></div>)}
        </div>
      </> : <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
        <div className="mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">好友申请会通过消息网关实时推送；离线后重新进入时由关系服务补拉。</div>
        {mockFriendRequests.map((request) => <div key={request.id} className="border-b border-slate-100 py-4"><div className="flex items-start gap-3"><UserAvatar src={request.avatar} name={request.name} size="md" /><div className="min-w-0 flex-1"><div className="flex items-center justify-between"><p className="text-sm font-semibold text-slate-800">{request.name}</p><span className="text-[11px] text-slate-400">{request.time}</span></div><p className="mt-0.5 text-xs text-slate-400">@{request.username} · {request.mutual} 位共同好友</p><p className="mt-2 rounded bg-slate-50 px-2.5 py-2 text-xs leading-5 text-slate-600">{request.message}</p>{handled[request.id] ? <p className={cn("mt-3 text-xs font-medium", handled[request.id] === "accepted" ? "text-emerald-600" : "text-slate-400")}>{handled[request.id] === "accepted" ? "已添加为好友" : "已忽略"}</p> : <div className="mt-3 flex gap-2"><button onClick={() => setHandled((state) => ({ ...state, [request.id]: "accepted" }))} className="flex h-8 items-center gap-1.5 rounded-md bg-emerald-600 px-3 text-xs font-medium text-white"><Check className="h-3.5 w-3.5" />接受</button><button onClick={() => setHandled((state) => ({ ...state, [request.id]: "rejected" }))} className="flex h-8 items-center gap-1.5 rounded-md border border-slate-200 px-3 text-xs text-slate-600"><X className="h-3.5 w-3.5" />忽略</button></div>}</div></div></div>)}
      </div>}
      {showAddFriend && <div className="absolute inset-0 z-40 flex items-center justify-center bg-slate-900/35 p-4"><div className="w-full max-w-sm rounded-lg bg-white p-5 shadow-xl"><div className="flex items-center justify-between"><h2 className="text-base font-semibold text-slate-900">添加好友</h2><button onClick={() => setShowAddFriend(false)} className="flex h-8 w-8 items-center justify-center rounded-md text-slate-400 hover:bg-slate-100" title="关闭"><X className="h-4 w-4" /></button></div><div className="relative mt-4"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input autoFocus placeholder="输入用户 ID、昵称或邮箱" className="h-10 w-full rounded-md border border-slate-200 pl-9 pr-3 text-sm outline-none focus:border-emerald-400" /></div><div className="mt-4 flex items-center gap-3 rounded-md border border-slate-100 bg-slate-50 p-3"><UserAvatar src="https://i.pravatar.cc/160?img=13" name="江澄" size="md" /><div className="min-w-0 flex-1"><p className="text-sm font-medium text-slate-800">江澄</p><p className="text-xs text-slate-400">@jiangcheng · SuIM-10028</p></div><button onClick={() => setRequestSent(true)} disabled={requestSent} className={cn("flex h-8 items-center gap-1.5 rounded-md px-3 text-xs font-medium", requestSent ? "bg-slate-200 text-slate-500" : "bg-emerald-600 text-white")}><Send className="h-3.5 w-3.5" />{requestSent ? "已申请" : "添加"}</button></div></div></div>}
    </div>
  );
}
