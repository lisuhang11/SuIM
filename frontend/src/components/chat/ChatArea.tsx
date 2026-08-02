"use client";

// ============================================================
// ChatArea — 聊天区域（Header + Messages + Input）
// ============================================================
import React, { useCallback, useEffect } from "react";
import type { Conversation, Message } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import ChatHeader from "./ChatHeader";
import MessageList from "./MessageList";
import MessageInput from "./MessageInput";

interface ChatAreaProps {
  conversation: Conversation;
  messages: Message[];
  onBack?: () => void;
}

export default function ChatArea({
  conversation,
  messages,
  onBack,
}: ChatAreaProps) {
  const { user } = useAuth();
  const {
    sendMessage,
    sendTyping,
    typingUsers,
    updateConversation,
    loadMessages,
    markConversationRead,
  } = useChat();

  useEffect(() => {
    void loadMessages(conversation.conversationId).then(() => {
      markConversationRead(conversation.conversationId);
    });
  }, [conversation.conversationId, loadMessages, markConversationRead]);

  const handleSend = useCallback(
    async (content: string) => {
      try {
        await sendMessage({
          conversationId: conversation.conversationId,
          content,
          type: "text",
        });
      } catch {
        // sendMessage 内部已标 failed；避免未捕获 Promise 打断 UI
      }
    },
    [conversation.conversationId, sendMessage]
  );

  const handleTyping = useCallback(
    (isTyping: boolean) => {
      sendTyping(conversation.conversationId, isTyping);
    },
    [conversation.conversationId, sendTyping]
  );

  const handleFile = useCallback(
    async (file: File, onProgress: (value: number) => void) => {
      try {
        const attachment = await IMSDK.uploadFile(file, onProgress);
        await sendMessage({
          conversationId: conversation.conversationId,
          content: attachment.name,
          type: attachment.category === "image" ? "image" : "file",
          file: attachment,
        });
      } catch {
        // 上传/发送失败时不抛到 UI；文本发送失败由 sendMessage 标 failed
      }
    },
    [conversation.conversationId, sendMessage]
  );

  const handleToggleMute = useCallback(
    async (next: boolean) => {
      const prev = Boolean(conversation.isMuted);
      if (next === prev) return;
      updateConversation(conversation.conversationId, { isMuted: next });
      if (!user?.userId) return;
      try {
        await IMSDK.setConversation(conversation, { isMuted: next }, user.userId);
      } catch {
        updateConversation(conversation.conversationId, { isMuted: prev });
      }
    },
    [conversation, updateConversation, user?.userId]
  );

  const handleTogglePin = useCallback(
    async (next: boolean) => {
      const prev = Boolean(conversation.isPinned);
      if (next === prev) return;
      updateConversation(conversation.conversationId, { isPinned: next });
      if (!user?.userId) return;
      try {
        await IMSDK.setConversation(conversation, { isPinned: next }, user.userId);
      } catch {
        updateConversation(conversation.conversationId, { isPinned: prev });
      }
    },
    [conversation, updateConversation, user?.userId]
  );

  const typingForThisConv = typingUsers[conversation.conversationId] || [];

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col bg-surface-elevated">
      <ChatHeader
        conversation={conversation}
        onBack={onBack}
        onToggleMute={handleToggleMute}
        onTogglePin={handleTogglePin}
      />
      <MessageList
        messages={messages}
        conversationId={conversation.conversationId}
        typingUsers={typingForThisConv}
      />
      <MessageInput
        onSend={handleSend}
        onFile={handleFile}
        onTyping={handleTyping}
      />
    </div>
  );
}
