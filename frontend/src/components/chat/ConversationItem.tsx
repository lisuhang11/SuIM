"use client";

import React from "react";
import { BellOff, CheckCheck, Pin, UsersRound } from "lucide-react";
import type { Conversation } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import { cn, formatConvTime, truncate } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

export default function ConversationItem({ conversation, isActive, onClick }: { conversation: Conversation; isActive: boolean; onClick: () => void }) {
  const { user } = useAuth();
  const { contacts, groups } = useChat();
  const otherId = conversation.members.find((item) => item.userId !== user?.userId)?.userId;
  const contact = contacts.find((item) => item.userId === otherId);
  const groupId =
    conversation.type === "group"
      ? conversation.groupId || IMSDK.parseGroupId(conversation.conversationId)
      : "";
  const group = groupId ? groups.find((item) => item.groupId === groupId) : undefined;
  const title =
    conversation.title ||
    group?.name ||
    contact?.displayName ||
    otherId ||
    "会话";
  const avatar = conversation.avatar || group?.avatar || contact?.avatar || "";
  const last = conversation.lastMessage;
  const mine = last?.senderId === user?.userId;

  return (
    <button onClick={onClick} className={cn("relative flex w-full items-center gap-3 px-4 py-3 text-left transition", isActive ? "bg-accent-soft/80" : "hover:bg-surface-muted")}>
      {isActive && <span className="absolute left-0 top-3 h-10 w-[3px] rounded-r bg-accent" />}
      <div className="relative">
        {conversation.type === "group" && !avatar ? (
          <div className="flex h-11 w-11 items-center justify-center rounded-control bg-accent-soft text-accent"><UsersRound className="h-5 w-5" strokeWidth={1.75} /></div>
        ) : <UserAvatar src={avatar} name={title} size="md" className="h-11 w-11" />}
        {contact && <span className={cn("absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-surface-elevated", contact.status === "online" ? "bg-accent" : contact.status === "away" ? "bg-amber-400" : "bg-ink-muted/40")} />}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <h3 className={cn("min-w-0 flex-1 truncate text-sm", conversation.unreadCount ? "font-semibold text-ink" : "font-medium text-ink")}>{title}</h3>
          {conversation.isPinned && <Pin className="h-3 w-3 text-amber-500" strokeWidth={1.75} />}
          <time className="text-[11px] text-ink-muted">{last ? formatConvTime(last.createdAt) : ""}</time>
        </div>
        <div className="mt-1 flex items-center gap-1.5">
          {mine && <CheckCheck className={cn("h-3.5 w-3.5", last?.status === "read" ? "text-accent" : "text-ink-muted/40")} strokeWidth={1.75} />}
          <p className="min-w-0 flex-1 truncate text-xs text-ink-muted">{last ? truncate(last.content, 34) : "暂无消息"}</p>
          {conversation.isMuted && <BellOff className="h-3.5 w-3.5 text-ink-muted" strokeWidth={1.75} />}
          {conversation.unreadCount > 0 && <span className="min-w-[19px] rounded-control bg-danger px-1.5 text-center text-[10px] font-semibold leading-[19px] text-white">{conversation.unreadCount}</span>}
        </div>
      </div>
    </button>
  );
}
