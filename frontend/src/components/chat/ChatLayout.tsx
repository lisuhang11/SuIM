"use client";

// ============================================================
// ChatLayout — 3 栏布局：图标导航 | 面板 | 聊天区域
// 三个栏目：会话 | 好友 | 群聊
// ============================================================
import React, { useState, useCallback } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import SidebarNav from "./SidebarNav";
import type { NavSection } from "./SidebarNav";
import ConversationList from "./ConversationList";
import FriendsPanel from "./FriendsPanel";
import GroupsPanel from "./GroupsPanel";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";

export default function ChatLayout() {
  const { user, isLoading } = useAuth();
  const { activeConversationId, messages, conversations } = useChat();
  const router = useRouter();
  const [navSection, setNavSection] = useState<NavSection>("chats");

  React.useEffect(() => {
    if (!isLoading && !user && process.env.NODE_ENV !== "development") {
      router.replace("/login");
    }
  }, [user, isLoading, router]);

  const handleNavigate = useCallback((section: NavSection) => {
    setNavSection(section);
  }, []);

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full" />
      </div>
    );
  }

  const activeConversation = activeConversationId
    ? conversations.find((c) => c.conversationId === activeConversationId) ?? null
    : null;
  const activeMessages = activeConversationId
    ? messages[activeConversationId] || []
    : [];

  return (
    <div className="h-screen flex bg-gray-50 overflow-hidden">
      {/* 栏 1: 图标导航 (64px) — 会话 | 好友 | 群聊 */}
      <SidebarNav activeSection={navSection} onNavigate={handleNavigate} />

      {/* 栏 2: 内容面板 (280px) — 桌面端始终可见 */}
      <div className="hidden md:flex w-72 flex-shrink-0 border-r border-gray-200 bg-white">
        {navSection === "chats" && <ConversationList />}
        {navSection === "friends" && <FriendsPanel />}
        {navSection === "groups" && <GroupsPanel />}
      </div>

      {/* 栏 3: 聊天区域 — flex-1 */}
      <div className="flex-1 flex flex-col min-w-0 bg-white">
        {activeConversation ? (
          <ChatArea
            conversation={activeConversation}
            messages={activeMessages}
            onBack={() => {}}
          />
        ) : (
          <EmptyChat />
        )}
      </div>
    </div>
  );
}
