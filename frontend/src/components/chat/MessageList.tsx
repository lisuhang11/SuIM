"use client";

import React, { useEffect, useRef } from "react";
import type { Message } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import MessageBubble from "./MessageBubble";
import TypingIndicator from "./TypingIndicator";

function dayLabel(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  if (sameDay(d, today)) return "今天";
  if (sameDay(d, yesterday)) return "昨天";
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

export default function MessageList({ messages, conversationId, typingUsers }: { messages: Message[]; conversationId: string; typingUsers: string[] }) {
  const { user } = useAuth();
  const { conversations, contacts } = useChat();
  const listRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);
  const conversation = conversations.find((item) => item.conversationId === conversationId);
  const isGroup = conversation?.type === "group";

  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    const onScroll = () => {
      stickToBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, [conversationId]);

  useEffect(() => {
    const el = listRef.current;
    if (!el || !stickToBottomRef.current) return;
    el.scrollTop = el.scrollHeight;
  }, [messages, typingUsers]);

  const resolveSender = (message: Message) => {
    if (message.senderId === user?.userId) {
      return {
        avatarSrc: message.senderAvatar || user?.avatar || "",
        displayName: message.senderName || user?.displayName || user?.username || "我",
      };
    }
    const contact = contacts.find((item) => item.userId === message.senderId);
    return {
      avatarSrc: message.senderAvatar || contact?.avatar || (conversation?.type === "private" ? conversation.avatar : "") || "",
      displayName: message.senderName || contact?.displayName || conversation?.title || message.senderId,
    };
  };

  return (
    <div ref={listRef} className="min-h-0 flex-1 overflow-y-auto bg-surface-muted py-5">
      {messages.length === 0 ? (
        <div className="flex h-full flex-col items-center justify-center text-center">
          <p className="text-sm font-medium text-ink-muted">还没有消息</p>
          <p className="mt-1 text-xs text-ink-muted">发送一条消息开始对话</p>
        </div>
      ) : (
        messages.map((message, index) => {
          const isMine = message.senderId === user?.userId;
          // 群聊昵称仅在同发送者连续消息的首条显示，头像每条都显示
          const showName =
            index === 0 || messages[index - 1]?.senderId !== message.senderId;
          const prevDay = index > 0 ? dayLabel(messages[index - 1].createdAt) : "";
          const curDay = dayLabel(message.createdAt);
          const showDay = curDay && curDay !== prevDay;
          const { avatarSrc, displayName } = resolveSender(message);
          return (
            <React.Fragment key={message.messageId}>
              {showDay && (
                <div className="mb-5 text-center">
                  <span className="text-[11px] font-medium text-ink-muted">{curDay}</span>
                </div>
              )}
              <MessageBubble
                message={message}
                isMine={isMine}
                isGroup={Boolean(isGroup)}
                showAvatar
                showName={showName}
                avatarSrc={avatarSrc}
                displayName={displayName}
              />
            </React.Fragment>
          );
        })
      )}
      {typingUsers.length > 0 && <TypingIndicator conversationId={conversationId} />}
    </div>
  );
}
