"use client";

import React, { useEffect, useRef } from "react";
import type { Message } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import MessageBubble from "./MessageBubble";
import TypingIndicator from "./TypingIndicator";

export default function MessageList({ messages, conversationId, typingUsers }: { messages: Message[]; conversationId: string; typingUsers: string[] }) {
  const { user } = useAuth();
  const { conversations } = useChat();
  const listRef = useRef<HTMLDivElement>(null);
  const isGroup = conversations.find((item) => item.conversationId === conversationId)?.type === "group";
  useEffect(() => {
    if (listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [messages, typingUsers]);

  return <div ref={listRef} className="min-h-0 flex-1 overflow-y-auto bg-[#f5f7f9] py-5">
    {messages.length > 0 && <div className="mb-5 text-center"><span className="text-[11px] font-medium text-slate-400">今天</span></div>}
    {messages.length === 0 ? <div className="flex h-full flex-col items-center justify-center text-center"><p className="text-sm font-medium text-slate-500">还没有消息</p><p className="mt-1 text-xs text-slate-400">发送一条消息开始对话</p></div> : messages.map((message, index) => <MessageBubble key={message.messageId} message={message} isMine={message.senderId === user?.userId} isGroup={Boolean(isGroup)} showAvatar={index === 0 || messages[index - 1]?.senderId !== message.senderId} />)}
    {typingUsers.length > 0 && <TypingIndicator conversationId={conversationId} />}
  </div>;
}
