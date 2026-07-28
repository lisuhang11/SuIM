"use client";

// ============================================================
// MessageBubble — 消息气泡
// ============================================================
import React from "react";
import { Check, CheckCheck, Clock } from "lucide-react";
import type { Message } from "@/types";
import { cn, formatTime } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

interface MessageBubbleProps {
  message: Message;
  isMine: boolean;
  isGroup: boolean;
  showAvatar: boolean;
}

export default function MessageBubble({ message, isMine, isGroup, showAvatar }: MessageBubbleProps) {
  const statusIcon = () => {
    switch (message.status) {
      case "sending":
        return <Clock className="w-3 h-3 text-gray-400" />;
      case "sent":
        return <Check className="w-3 h-3 text-gray-400" />;
      case "delivered":
        return <CheckCheck className="w-3 h-3 text-gray-400" />;
      case "read":
        return <CheckCheck className="w-3 h-3 text-blue-400" />;
      case "failed":
        return <span className="text-xs text-red-500">发送失败</span>;
    }
  };

  return (
    <div
      className={cn(
        "flex gap-2 mb-4 px-4",
        isMine ? "flex-row-reverse" : "flex-row"
      )}
    >
      {/* Avatar */}
      {isGroup && !isMine ? (
        showAvatar ? (
          <UserAvatar name={message.senderName} size="sm" className="mt-1" />
        ) : (
          <div className="w-8 h-8 flex-shrink-0" />
        )
      ) : null}

      {/* Content */}
      <div
        className={cn(
          "flex flex-col max-w-[70%]",
          isMine ? "items-end" : "items-start"
        )}
      >
        {/* Sender name (group chat) */}
        {isGroup && !isMine && showAvatar && (
          <span className="text-xs text-gray-400 mb-0.5 ml-1">
            {message.senderName}
          </span>
        )}

        {/* Bubble */}
        <div
          className={cn(
            "px-4 py-2.5 rounded-2xl relative break-words text-[15px] leading-relaxed",
            isMine
              ? "bg-indigo-500 text-white rounded-br-md"
              : "bg-white text-gray-800 rounded-bl-md shadow-sm border border-gray-100"
          )}
        >
          {/* System message */}
          {message.type === "system" ? (
            <span className="text-gray-400 italic text-xs">{message.content}</span>
          ) : (
            <>{message.content}</>
          )}
        </div>

        {/* Time + Status */}
        <div
          className={cn(
            "flex items-center gap-1 mt-1",
            isMine && "flex-row-reverse"
          )}
        >
          <span className="text-[11px] text-gray-400">
            {formatTime(message.createdAt)}
          </span>
          {isMine && message.type !== "system" && statusIcon()}
        </div>
      </div>
    </div>
  );
}
