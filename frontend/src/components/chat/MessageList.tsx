"use client";

// ============================================================
// MessageList — 消息列表（虚拟滚动优化）
// ============================================================
import React, { useEffect, useRef } from "react";
import type { Message } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { mockConversations } from "@/data/mock";
import MessageBubble from "./MessageBubble";
import TypingIndicator from "./TypingIndicator";

interface MessageListProps {
  messages: Message[];
  conversationId: string;
  typingUsers: string[];
}

export default function MessageList({
  messages,
  conversationId,
  typingUsers,
}: MessageListProps) {
  const { user } = useAuth();
  const bottomRef = useRef<HTMLDivElement>(null);

  // 判断是否为群聊
  const conv = mockConversations.find((c) => c.conversationId === conversationId);
  const isGroup = conv?.type === "group";

  // 自动滚动到底部
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, typingUsers]);

  // 判断是否需要显示头像（群聊中连续消息合并）
  const shouldShowAvatar = (index: number): boolean => {
    if (index === 0) return true;
    const prev = messages[index - 1];
    return prev?.senderId !== messages[index].senderId;
  };

  return (
    <div className="flex-1 overflow-y-auto bg-gray-50 py-2">
      {messages.length === 0 ? (
        <div className="flex items-center justify-center h-full text-gray-400 text-sm">
          暂无消息，发送第一条消息吧
        </div>
      ) : (
        messages.map((msg, idx) => (
          <MessageBubble
            key={msg.messageId}
            message={msg}
            isMine={msg.senderId === user?.userId}
            isGroup={isGroup}
            showAvatar={shouldShowAvatar(idx)}
          />
        ))
      )}

      {/* 正在输入 */}
      {typingUsers.length > 0 && (
        <TypingIndicator conversationId={conversationId} />
      )}

      {/* 滚动锚点 */}
      <div ref={bottomRef} />
    </div>
  );
}
