"use client";

// ============================================================
// ChatArea — 聊天区域（Header + Messages + Input）
// ============================================================
import React, { useCallback } from "react";
import type { Conversation, Message } from "@/types";
import { useChat } from "@/contexts/ChatContext";
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
  const {
    sendMessage,
    sendTyping,
    typingUsers,
    removeConversation,
    addConversation,
  } = useChat();

  const handleSend = useCallback(
    async (content: string) => {
      await sendMessage({
        conversationId: conversation.conversationId,
        content,
        type: "text",
      });
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
      const { uploadFile } = await import("@/services/api");
      const attachment = await uploadFile(file, onProgress);
      await sendMessage({
        conversationId: conversation.conversationId,
        content: attachment.name,
        type: attachment.category === "image" ? "image" : "file",
        file: attachment,
      });
    },
    [conversation.conversationId, sendMessage]
  );

  const handleToggleMute = () => {
    addConversation({ ...conversation, isMuted: !conversation.isMuted });
  };

  const handleTogglePin = () => {
    addConversation({ ...conversation, isPinned: !conversation.isPinned });
  };

  const typingForThisConv = typingUsers[conversation.conversationId] || [];

  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col bg-white">
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
