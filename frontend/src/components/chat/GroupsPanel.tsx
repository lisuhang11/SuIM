"use client";

import React, { useState } from "react";
import { ArrowLeft, BellOff, Check, ChevronRight, Plus, Search, ShieldCheck, UsersRound, X } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import CreateGroupDialog from "./CreateGroupDialog";
import UserAvatar from "../shared/UserAvatar";

interface GroupsPanelProps { onOpenConversation?: () => void }

export default function GroupsPanel({ onOpenConversation }: GroupsPanelProps) {
  const { groups, conversations, setActiveConversation } = useChat();
  const [query, setQuery] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [showApplications, setShowApplications] = useState(false);
  const [applicationStatus, setApplicationStatus] = useState<"pending" | "accepted" | "rejected">("pending");
  const filtered = groups.filter((item) => item.name.toLowerCase().includes(query.toLowerCase()));
  const openGroup = (groupId: string) => {
    const conversation = conversations.find((item) => item.conversationId === groupId);
    if (conversation) { setActiveConversation(conversation.conversationId); onOpenConversation?.(); }
  };

  return <div className="flex h-full w-full flex-col bg-white">
    <header className="px-5 pb-3 pt-5"><div className="flex items-center justify-between"><div><p className="text-xs font-medium text-slate-400">GROUPS</p><h1 className="mt-1 text-xl font-semibold text-slate-900">群组</h1></div><button onClick={() => setShowCreate(true)} className="flex h-9 w-9 items-center justify-center rounded-md border border-slate-200 text-slate-600" title="创建群组"><Plus className="h-4 w-4" /></button></div><div className="relative mt-4"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索群组" className="h-10 w-full rounded-md border border-slate-200 bg-slate-50 pl-9 pr-3 text-sm outline-none focus:border-emerald-400" /></div></header>
    {!showApplications && <div className="mx-5 mb-3 flex items-center justify-between rounded-md border border-sky-200 bg-sky-50 px-3 py-2.5"><div><p className="text-xs font-semibold text-sky-800">{applicationStatus === "pending" ? "1 条入群申请待处理" : "入群申请已处理"}</p><p className="mt-0.5 text-[11px] text-sky-600">SuIM 产品与研发</p></div><button onClick={() => setShowApplications(true)} className="text-xs font-medium text-sky-700">{applicationStatus === "pending" ? "处理" : "查看"}</button></div>}
    {showApplications ? <div className="min-h-0 flex-1 overflow-y-auto px-5"><button onClick={() => setShowApplications(false)} className="mb-4 flex items-center gap-1 text-xs font-medium text-slate-500"><ArrowLeft className="h-3.5 w-3.5" />返回群组</button><h2 className="text-sm font-semibold text-slate-900">入群申请</h2><div className="mt-4 rounded-md border border-slate-200 p-4"><div className="flex items-start gap-3"><UserAvatar src="https://i.pravatar.cc/160?img=8" name="罗言" size="md" /><div className="min-w-0 flex-1"><div className="flex items-center justify-between"><p className="text-sm font-medium text-slate-800">罗言</p><span className="text-[11px] text-slate-400">32 分钟前</span></div><p className="mt-0.5 text-xs text-slate-400">申请加入「SuIM 产品与研发」</p><p className="mt-3 rounded bg-slate-50 px-2.5 py-2 text-xs leading-5 text-slate-600">正在学习消息队列，希望参与后端架构讨论。</p>{applicationStatus === "pending" ? <div className="mt-3 flex gap-2"><button onClick={() => setApplicationStatus("accepted")} className="flex h-8 items-center gap-1.5 rounded-md bg-emerald-600 px-3 text-xs font-medium text-white"><Check className="h-3.5 w-3.5" />同意</button><button onClick={() => setApplicationStatus("rejected")} className="flex h-8 items-center gap-1.5 rounded-md border border-slate-200 px-3 text-xs text-slate-600"><X className="h-3.5 w-3.5" />拒绝</button></div> : <p className={`mt-3 text-xs font-medium ${applicationStatus === "accepted" ? "text-emerald-600" : "text-slate-400"}`}>{applicationStatus === "accepted" ? "已同意加入" : "已拒绝"}</p>}</div></div></div></div> : <div className="min-h-0 flex-1 overflow-y-auto"><p className="px-5 py-2 text-[11px] font-semibold uppercase text-slate-400">我加入的群组 · {groups.length}</p>{filtered.map((group) => <button key={group.groupId} onClick={() => openGroup(group.groupId)} className="flex w-full items-center gap-3 px-5 py-3.5 text-left hover:bg-slate-50"><div className="flex h-11 w-11 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-5 w-5" /></div><div className="min-w-0 flex-1"><div className="flex items-center gap-1.5"><p className="truncate text-sm font-medium text-slate-800">{group.name}</p>{group.ownerId === "u_10001" && <ShieldCheck className="h-3.5 w-3.5 text-amber-500" />}{group.isMuted && <BellOff className="h-3.5 w-3.5 text-slate-400" />}</div><p className="mt-0.5 truncate text-xs text-slate-400">{group.memberCount} 名成员 · {group.notification || group.introduction}</p></div><ChevronRight className="h-4 w-4 text-slate-300" /></button>)}</div>}
    {showCreate && <CreateGroupDialog onClose={() => setShowCreate(false)} />}
  </div>;
}
