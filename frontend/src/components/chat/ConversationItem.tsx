"use client";

import React from "react";
import { BellOff, CheckCheck, Pin, UsersRound } from "lucide-react";
import type { Conversation } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { mockContacts } from "@/services/mock-data";
import { cn, formatConvTime, truncate } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

export default function ConversationItem({ conversation, isActive, onClick }: { conversation: Conversation; isActive: boolean; onClick: () => void }) {
  const { user } = useAuth();
  const otherId = conversation.members.find((item) => item.userId !== user?.userId)?.userId;
  const contact = mockContacts.find((item) => item.userId === otherId);
  const last = conversation.lastMessage;
  const mine = last?.senderId === user?.userId;

  return (
    <button onClick={onClick} className={cn("relative flex w-full items-center gap-3 px-4 py-3 text-left transition", isActive ? "bg-emerald-50/80" : "hover:bg-slate-50")}>
      {isActive && <span className="absolute left-0 top-3 h-10 w-[3px] rounded-r bg-emerald-500" />}
      <div className="relative">
        {conversation.type === "group" && !conversation.avatar ? (
          <div className="flex h-11 w-11 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-5 w-5" /></div>
        ) : <UserAvatar src={conversation.avatar} name={conversation.title} size="md" className="h-11 w-11" />}
        {contact && <span className={cn("absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-white", contact.status === "online" ? "bg-emerald-500" : contact.status === "away" ? "bg-amber-400" : "bg-slate-300")} />}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <h3 className={cn("min-w-0 flex-1 truncate text-sm", conversation.unreadCount ? "font-semibold text-slate-900" : "font-medium text-slate-700")}>{conversation.title}</h3>
          {conversation.isPinned && <Pin className="h-3 w-3 text-amber-500" />}
          <time className="text-[11px] text-slate-400">{last ? formatConvTime(last.createdAt) : ""}</time>
        </div>
        <div className="mt-1 flex items-center gap-1.5">
          {mine && <CheckCheck className={cn("h-3.5 w-3.5", last?.status === "read" ? "text-emerald-500" : "text-slate-300")} />}
          <p className="min-w-0 flex-1 truncate text-xs text-slate-400">{last ? truncate(last.content, 34) : "暂无消息"}</p>
          {conversation.isMuted && <BellOff className="h-3.5 w-3.5 text-slate-400" />}
          {conversation.unreadCount > 0 && <span className="min-w-[19px] rounded-full bg-rose-500 px-1.5 text-center text-[10px] font-semibold leading-[19px] text-white">{conversation.unreadCount}</span>}
        </div>
      </div>
    </button>
  );
}
