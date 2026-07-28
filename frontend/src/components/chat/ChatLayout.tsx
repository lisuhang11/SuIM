"use client";

import React, { useCallback, useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import SidebarNav, { type NavSection } from "./SidebarNav";
import ConversationList from "./ConversationList";
import FriendsPanel from "./FriendsPanel";
import GroupsPanel from "./GroupsPanel";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";

export default function ChatLayout() {
  const { isLoading } = useAuth();
  const { activeConversationId, messages, conversations } = useChat();
  const [navSection, setNavSection] = useState<NavSection>("chats");
  const [mobileChatOpen, setMobileChatOpen] = useState(false);

  const handleNavigate = useCallback((section: NavSection) => {
    setNavSection(section);
    setMobileChatOpen(false);
  }, []);

  if (isLoading) {
    return <div className="flex h-dvh items-center justify-center bg-slate-100"><div className="h-7 w-7 animate-spin rounded-full border-2 border-emerald-500 border-t-transparent" /></div>;
  }

  const activeConversation = conversations.find((item) => item.conversationId === activeConversationId) || null;
  const activeMessages = activeConversationId ? messages[activeConversationId] || [] : [];

  return (
    <main className="flex h-dvh flex-col-reverse overflow-hidden bg-slate-100 md:flex-row">
      <SidebarNav activeSection={navSection} onNavigate={handleNavigate} />
      <section className={`${mobileChatOpen ? "hidden" : "flex"} min-h-0 min-w-0 flex-1 border-r border-slate-200 bg-white md:flex md:w-[340px] md:flex-none`}>
        {navSection === "chats" && <ConversationList onOpenConversation={() => setMobileChatOpen(true)} />}
        {navSection === "friends" && <FriendsPanel onOpenConversation={() => setMobileChatOpen(true)} />}
        {navSection === "groups" && <GroupsPanel onOpenConversation={() => setMobileChatOpen(true)} />}
      </section>
      <section className={`${mobileChatOpen ? "flex" : "hidden"} min-h-0 min-w-0 flex-1 bg-[#f5f7f9] md:flex`}>
        {activeConversation ? (
          <ChatArea conversation={activeConversation} messages={activeMessages} onBack={() => setMobileChatOpen(false)} />
        ) : (
          <EmptyChat />
        )}
      </section>
    </main>
  );
}
