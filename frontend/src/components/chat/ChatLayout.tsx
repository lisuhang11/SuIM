"use client";

// ============================================================
// ChatLayout — 聊天主布局
// ============================================================
import React from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import ConversationList from "./ConversationList";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";
import { getConversationById, getMessagesByConversationId } from "@/data/mock";

export default function ChatLayout() {
  const { user, isLoading } = useAuth();
  const { activeConversationId, messages } = useChat();
  const router = useRouter();

  // 开发模式自动跳过认证检查，但保留守卫逻辑
  React.useEffect(() => {
    if (!isLoading && !user && process.env.NODE_ENV !== "development") {
      router.replace("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full" />
      </div>
    );
  }

  const activeConversation = activeConversationId
    ? getConversationById(activeConversationId)
    : null;

  const activeMessages = activeConversationId
    ? messages[activeConversationId] || getMessagesByConversationId(activeConversationId)
    : [];

  return (
    <div className="h-screen flex bg-gray-50 overflow-hidden">
      {/* 左侧会话列表 */}
      <div
        className="w-full md:w-80 lg:w-96 flex-shrink-0 border-r border-gray-200 bg-white"
        style={{
          display: activeConversation ? undefined : "block",
        }}
      >
        <div
          className={activeConversation ? "hidden md:block h-full" : "h-full"}
        >
          <ConversationList />
        </div>
      </div>

      {/* 右侧聊天区域 */}
      <div
        className="flex-1 flex flex-col min-w-0 bg-white"
        style={{
          display: activeConversation ? "flex" : "none",
        }}
      >
        {activeConversation ? (
          <ChatArea
            conversation={activeConversation}
            messages={activeMessages}
            onBack={() => {
              // 移动端返回会话列表
              // setActiveConversation(null)
            }}
          />
        ) : (
          <div className="hidden md:flex flex-1">
            <EmptyChat />
          </div>
        )}
      </div>

      {/* Desktop: 默认显示空状态 */}
      <div
        className="hidden md:flex flex-1"
        style={{
          display: !activeConversation ? "flex" : "none",
        }}
      >
        <EmptyChat />
      </div>
    </div>
  );
}
