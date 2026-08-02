"use client";

import React, { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import SidebarNav, { type NavSection } from "./SidebarNav";
import ConversationList from "./ConversationList";
import FriendsPanel from "./FriendsPanel";
import GroupsPanel from "./GroupsPanel";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";
import ProfilePanel from "./ProfilePanel";

export default function ChatLayout() {
  const router = useRouter();
  const { isLoading, isAuthenticated } = useAuth();
  const { activeConversationId, messages, conversations } = useChat();
  const [navSection, setNavSection] = useState<NavSection>("chats");
  const [mobileChatOpen, setMobileChatOpen] = useState(false);
  const [profileOpen, setProfileOpen] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.replace("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  const handleNavigate = useCallback((section: NavSection) => {
    setNavSection(section);
    setMobileChatOpen(false);
  }, []);

  if (isLoading || !isAuthenticated) {
    return (
      <div className="flex h-dvh items-center justify-center bg-surface">
        <div className="h-7 w-7 animate-spin rounded-full border-2 border-accent border-t-transparent" />
      </div>
    );
  }

  const activeConversation =
    conversations.find((item) => item.conversationId === activeConversationId) || null;
  const activeMessages = activeConversationId ? messages[activeConversationId] || [] : [];

  return (
    <main className="flex h-dvh flex-col-reverse overflow-hidden bg-surface md:flex-row">
      <SidebarNav
        activeSection={navSection}
        onNavigate={handleNavigate}
        onOpenProfile={() => setProfileOpen(true)}
      />
      <section
        className={`${
          mobileChatOpen ? "hidden" : "flex"
        } min-h-0 min-w-0 flex-1 border-r border-edge bg-surface-elevated md:flex md:w-[320px] md:flex-none`}
      >
        {navSection === "chats" && (
          <ConversationList onOpenConversation={() => setMobileChatOpen(true)} />
        )}
        {navSection === "friends" && (
          <FriendsPanel onOpenConversation={() => setMobileChatOpen(true)} />
        )}
        {navSection === "groups" && (
          <GroupsPanel onOpenConversation={() => setMobileChatOpen(true)} />
        )}
      </section>
      <section
        className={`${
          mobileChatOpen ? "flex" : "hidden"
        } min-h-0 min-w-0 flex-1 bg-surface md:flex`}
      >
        {activeConversation ? (
          <ChatArea
            conversation={activeConversation}
            messages={activeMessages}
            onBack={() => setMobileChatOpen(false)}
          />
        ) : (
          <EmptyChat />
        )}
      </section>
      <ProfilePanel open={profileOpen} onClose={() => setProfileOpen(false)} />
    </main>
  );
}
