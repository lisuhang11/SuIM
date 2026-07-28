"use client";

import React, { useState } from "react";
import { ArrowLeft, Bell, BellOff, Info, MoreHorizontal, Phone, Pin, Search, UsersRound, Video, X } from "lucide-react";
import type { Conversation } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

export default function ChatHeader({ conversation, onBack, onToggleMute, onTogglePin }: { conversation: Conversation; onBack?: () => void; onToggleMute?: () => void; onTogglePin?: () => void }) {
  const { user } = useAuth();
  const { contacts } = useChat();
  const [showInfo, setShowInfo] = useState(false);
  const otherId = conversation.members.find((item) => item.userId !== user?.userId)?.userId;
  const other = contacts.find((item) => item.userId === otherId);
  const status = conversation.type === "group" ? `${conversation.members.length} 名成员` : other?.status === "online" ? "在线" : other?.status === "away" ? "离开" : "离线";
  const iconButton = "flex h-9 w-9 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-slate-800";

  return <>
    <header className="flex h-[68px] flex-shrink-0 items-center justify-between border-b border-slate-200 bg-white px-3 sm:px-5">
      <div className="flex min-w-0 items-center gap-2.5">
        <button onClick={onBack} className={`${iconButton} md:hidden`} title="返回"><ArrowLeft className="h-5 w-5" /></button>
        {conversation.type === "group" && !conversation.avatar ? <div className="flex h-10 w-10 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-5 w-5" /></div> : <UserAvatar src={conversation.avatar} name={conversation.title} size="md" />}
        <div className="min-w-0"><div className="flex items-center gap-2"><h2 className="truncate text-sm font-semibold text-slate-900">{conversation.title}</h2>{conversation.isPinned && <Pin className="h-3.5 w-3.5 text-amber-500" />}</div><p className="mt-0.5 flex items-center gap-1.5 text-xs text-slate-400"><span className={cn("h-1.5 w-1.5 rounded-full", status === "在线" ? "bg-emerald-500" : "bg-slate-300")} />{status}</p></div>
      </div>
      <div className="flex items-center gap-0.5">
        <button className={`${iconButton} hidden sm:flex`} title="搜索聊天记录"><Search className="h-[18px] w-[18px]" /></button>
        <button onClick={() => window.alert(`正在呼叫 ${conversation.title}`)} className={iconButton} title="语音通话"><Phone className="h-[18px] w-[18px]" /></button>
        <button onClick={() => window.alert(`正在邀请 ${conversation.title} 进行视频通话`)} className={`${iconButton} hidden sm:flex`} title="视频通话"><Video className="h-[18px] w-[18px]" /></button>
        <button onClick={() => setShowInfo(true)} className={cn(iconButton, showInfo && "bg-slate-100 text-slate-900")} title="会话详情"><Info className="h-[18px] w-[18px]" /></button>
        <button className={iconButton} title="更多操作"><MoreHorizontal className="h-[18px] w-[18px]" /></button>
      </div>
    </header>

    {showInfo && <aside className="absolute bottom-0 right-0 top-[68px] z-20 w-full border-l border-slate-200 bg-white shadow-xl sm:w-[300px]">
      <div className="flex h-14 items-center justify-between border-b border-slate-100 px-5"><h3 className="text-sm font-semibold text-slate-900">会话详情</h3><button onClick={() => setShowInfo(false)} className={iconButton} title="关闭"><X className="h-4 w-4" /></button></div>
      <div className="overflow-y-auto p-5">
        <div className="flex flex-col items-center border-b border-slate-100 pb-5">{conversation.type === "group" && !conversation.avatar ? <div className="flex h-16 w-16 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-7 w-7" /></div> : <UserAvatar src={conversation.avatar} name={conversation.title} size="xl" />}<p className="mt-3 text-sm font-semibold text-slate-900">{conversation.title}</p><p className="mt-1 text-xs text-slate-400">{status}</p></div>
        <div className="border-b border-slate-100 py-4"><p className="mb-3 text-xs font-semibold text-slate-500">成员</p><div className="grid grid-cols-4 gap-3">{conversation.members.slice(0, 8).map((item) => { const contact = contacts.find((entry) => entry.userId === item.userId); const name = item.userId === user?.userId ? "我" : contact?.displayName || item.userId; return <div key={item.userId} className="min-w-0 text-center"><UserAvatar src={item.userId === user?.userId ? user.avatar : contact?.avatar} name={name} size="md" className="mx-auto" /><p className="mt-1 truncate text-[11px] text-slate-500">{name}</p></div>; })}</div></div>
        <div className="py-3"><button onClick={onTogglePin} className="flex h-11 w-full items-center justify-between text-sm text-slate-700"><span className="flex items-center gap-2"><Pin className="h-4 w-4 text-slate-400" />置顶会话</span><span className={cn("h-5 w-9 rounded-full p-0.5 transition", conversation.isPinned ? "bg-emerald-500" : "bg-slate-200")}><span className={cn("block h-4 w-4 rounded-full bg-white transition", conversation.isPinned && "translate-x-4")} /></span></button><button onClick={onToggleMute} className="flex h-11 w-full items-center justify-between text-sm text-slate-700"><span className="flex items-center gap-2">{conversation.isMuted ? <BellOff className="h-4 w-4 text-slate-400" /> : <Bell className="h-4 w-4 text-slate-400" />}消息免打扰</span><span className={cn("h-5 w-9 rounded-full p-0.5 transition", conversation.isMuted ? "bg-emerald-500" : "bg-slate-200")}><span className={cn("block h-4 w-4 rounded-full bg-white transition", conversation.isMuted && "translate-x-4")} /></span></button></div>
      </div>
    </aside>}
  </>;
}
