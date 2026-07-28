"use client";

// ============================================================
// ChatLayout — 3 栏布局：图标导航 | 面板 | 聊天区域
// ============================================================
import React, { useState, useCallback } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import SidebarNav from "./SidebarNav";
import type { NavSection } from "./SidebarNav";
import ConversationList from "./ConversationList";
import ContactsPanel from "./ContactsPanel";
import AddFriendPanel from "./AddFriendPanel";
import FriendRequestsPanel from "./FriendRequestsPanel";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";
import { getConversationById, getMessagesByConversationId } from "@/data/mock";

export default function ChatLayout() {
  const { user, isLoading } = useAuth();
  const { activeConversationId, messages } = useChat();
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
    ? getConversationById(activeConversationId)
    : null;
  const activeMessages = activeConversationId
    ? messages[activeConversationId] || getMessagesByConversationId(activeConversationId)
    : [];

  const showPanel = !activeConversation || true; // 桌面端始终显示面板

  return (
    <div className="h-screen flex bg-gray-50 overflow-hidden">
      {/* 栏 1: 图标导航 (64px) */}
      <SidebarNav activeSection={navSection} onNavigate={handleNavigate} />

      {/* 栏 2: 内容面板 (280px) — 桌面端始终可见 */}
      <div
        className="hidden md:flex w-72 flex-shrink-0 border-r border-gray-200 bg-white"
      >
        {navSection === "chats" && <ConversationList />}
        {navSection === "contacts" && <ContactsPanel />}
        {navSection === "requests" && <FriendRequestsPanel />}
        {navSection === "settings" && (
          <div className="flex-1 flex items-center justify-center text-gray-400 text-sm">
            设置页面（开发中）
          </div>
        )}
      </div>

      {/* 栏 3: 聊天区域 — flex-1 */}
      <div className="flex-1 flex flex-col min-w-0 bg-white">
        {activeConversation ? (
          <ChatArea
            conversation={activeConversation}
            messages={activeMessages}
            onBack={() => {
              // 移动端返回面板
            }}
          />
        ) : (
          <EmptyChat />
        )}
      </div>
    </div>
  );
}
