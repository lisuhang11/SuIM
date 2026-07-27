"use client";

// ============================================================
// ChatHeader — 聊天窗口顶部栏
// ============================================================
import React, { useState } from "react";
import {
  Phone,
  Video,
  Info,
  Bell,
  BellOff,
  Pin,
  Search,
  MoreHorizontal,
  ArrowLeft,
} from "lucide-react";
import type { Conversation, User } from "@/types";
import { getUserById } from "@/data/mock";
import { getStatusText } from "@/data/mock";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { cn } from "@/lib/utils";

interface ChatHeaderProps {
  conversation: Conversation;
  onBack?: () => void;
  onToggleMute?: () => void;
  onTogglePin?: () => void;
}

export default function ChatHeader({
  conversation,
  onBack,
  onToggleMute,
  onTogglePin,
}: ChatHeaderProps) {
  const [showInfo, setShowInfo] = useState(false);

  // 私聊显示对方信息
  const otherMember = conversation.type === "private"
    ? conversation.members.find((m) => m.userId !== "u_1001")
    : null;
  const otherUser = otherMember ? getUserById(otherMember.userId) : undefined;

  const statusText = conversation.type === "group"
    ? `${conversation.members.length} 名成员`
    : getStatusText(otherUser?.status || "offline");

  const online = otherUser?.status === "online";

  return (
    <>
      <div className="h-16 flex items-center justify-between px-4 border-b border-gray-200 bg-white flex-shrink-0">
        <div className="flex items-center gap-3 min-w-0">
          {/* 返回按钮（移动端） */}
          {onBack && (
            <button
              onClick={onBack}
              className="p-1.5 rounded-lg hover:bg-gray-100 transition-colors md:hidden"
            >
              <ArrowLeft className="w-5 h-5 text-gray-500" />
            </button>
          )}

          {/* 头像 + 状态 */}
          <button
            onClick={() => setShowInfo(!showInfo)}
            className="flex items-center gap-3 min-w-0 hover:bg-gray-50 rounded-xl px-2 py-1 transition-colors"
          >
            <div className="relative flex-shrink-0">
              <UserAvatar
                src={conversation.avatar || otherUser?.avatar}
                name={conversation.title}
                size="md"
              />
              {conversation.type === "private" && otherUser && (
                <OnlineBadge
                  status={otherUser.status}
                  size="sm"
                  className="absolute -bottom-0.5 -right-0.5"
                />
              )}
            </div>
            <div className="min-w-0 text-left">
              <h2 className="font-semibold text-gray-900 truncate text-sm">
                {conversation.title}
              </h2>
              <p className="text-xs text-gray-400 truncate">{statusText}</p>
            </div>
          </button>
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-1">
          <button className="p-2 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
            <Phone className="w-4 h-4" />
          </button>
          <button className="p-2 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
            <Video className="w-4 h-4" />
          </button>
          <button className="hidden sm:block p-2 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
            <Search className="w-4 h-4" />
          </button>
          <button
            onClick={onToggleMute}
            className={cn(
              "p-2 rounded-lg transition-colors",
              conversation.isMuted
                ? "text-amber-500 bg-amber-50"
                : "text-gray-400 hover:text-gray-600 hover:bg-gray-100"
            )}
            title={conversation.isMuted ? "取消免打扰" : "免打扰"}
          >
            {conversation.isMuted ? (
              <BellOff className="w-4 h-4" />
            ) : (
              <Bell className="w-4 h-4" />
            )}
          </button>
          <button
            onClick={onTogglePin}
            className={cn(
              "p-2 rounded-lg transition-colors",
              conversation.isPinned
                ? "text-indigo-500 bg-indigo-50"
                : "text-gray-400 hover:text-gray-600 hover:bg-gray-100"
            )}
            title={conversation.isPinned ? "取消置顶" : "置顶"}
          >
            <Pin className="w-4 h-4" />
          </button>
          <button className="p-2 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors">
            <MoreHorizontal className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* 会话信息面板 */}
      {showInfo && (
        <div className="bg-gray-50 border-b border-gray-200 px-4 py-3 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-gray-500">
              {conversation.type === "group" ? "群聊成员" : "好友信息"}
            </span>
            <button
              onClick={() => setShowInfo(false)}
              className="text-gray-400 hover:text-gray-600"
            >
              收起
            </button>
          </div>
          <div className="mt-2 flex flex-wrap gap-2">
            {conversation.members.map((m) => {
              const u = getUserById(m.userId);
              return u ? (
                <div
                  key={m.userId}
                  className="flex items-center gap-1.5 bg-white rounded-lg px-2 py-1 shadow-sm"
                >
                  <UserAvatar name={u.displayName} size="sm" />
                  <span className="text-xs text-gray-700">{u.displayName}</span>
                  {conversation.type === "group" && m.role !== "member" && (
                    <span className="text-[10px] bg-indigo-100 text-indigo-600 px-1 rounded">
                      {m.role === "owner" ? "群主" : "管理"}
                    </span>
                  )}
                </div>
              ) : null;
            })}
          </div>
        </div>
      )}
    </>
  );
}
