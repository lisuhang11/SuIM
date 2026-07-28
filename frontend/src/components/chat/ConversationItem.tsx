"use client";

// ============================================================
// ConversationItem — 单个会话条目
// ============================================================
import React from "react";
import { Pin, BellOff, CheckCheck, Check } from "lucide-react";
import type { Conversation } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { cn, formatConvTime, truncate } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";

interface ConversationItemProps {
  conversation: Conversation;
  isActive: boolean;
  onClick: () => void;
}

export default function ConversationItem({
  conversation,
  isActive,
  onClick,
}: ConversationItemProps) {
  const { user } = useAuth();

  // 私聊显示对方信息
  const otherMember = conversation.type === "private"
    ? conversation.members.find((m) => m.userId !== user?.userId)
    : null;

  const lastMsg = conversation.lastMessage;
  const isMyLastMsg = lastMsg?.senderId === user?.userId;

  // 最后一条消息预览
  const preview = lastMsg
    ? `${isMyLastMsg ? "我: " : ""}${truncate(lastMsg.content, 30)}`
    : "暂无消息";

  // 最后消息状态图标
  const StatusIcon = lastMsg?.status === "read" ? CheckCheck : Check;

  return (
    <button
      onClick={onClick}
      className={cn(
        "w-full flex items-center gap-3 px-4 py-3 text-left transition-colors",
        "hover:bg-gray-50 dark:hover:bg-gray-750",
        isActive && "bg-indigo-50 hover:bg-indigo-50"
      )}
    >
      {/* 头像 */}
      <div className="relative flex-shrink-0">
        <UserAvatar
          src={conversation.avatar}
          name={conversation.title}
          size="md"
        />
        {conversation.type === "private" && otherMember && (
          <OnlineBadge
            status="offline"
            size="sm"
            className="absolute -bottom-0.5 -right-0.5"
          />
        )}
      </div>

      {/* 内容 */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 min-w-0">
            <h3 className={cn(
              "font-medium truncate text-sm",
              conversation.unreadCount > 0 ? "text-gray-900" : "text-gray-700"
            )}>
              {conversation.title}
            </h3>
            {conversation.isPinned && (
              <Pin className="w-3 h-3 text-amber-500 flex-shrink-0" />
            )}
          </div>
          <span className="text-[11px] text-gray-400 flex-shrink-0 ml-2">
            {lastMsg ? formatConvTime(lastMsg.createdAt) : ""}
          </span>
        </div>

        <div className="flex items-center justify-between mt-0.5">
          <p className={cn(
            "text-xs truncate",
            conversation.unreadCount > 0 ? "text-gray-600 font-medium" : "text-gray-400"
          )}>
            {conversation.isMuted && (
              <BellOff className="w-3 h-3 inline mr-1 text-gray-400" />
            )}
            {preview}
          </p>
          {conversation.unreadCount > 0 ? (
            <span className="flex-shrink-0 bg-indigo-500 text-white text-[10px] font-semibold
              min-w-[18px] h-[18px] rounded-full flex items-center justify-center px-1 ml-2">
              {conversation.unreadCount > 99 ? "99+" : conversation.unreadCount}
            </span>
          ) : isMyLastMsg ? (
            <StatusIcon className="w-3 h-3 text-gray-300 flex-shrink-0 ml-2" />
          ) : null}
        </div>
      </div>
    </button>
  );
}
