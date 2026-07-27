"use client";

// ============================================================
// ContactsPanel — 联系人面板
// ============================================================
import React, { useState } from "react";
import { UserPlus, MessageSquare } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import type { Contact } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { getStatusText } from "@/data/mock";

export default function ContactsPanel() {
  const { contacts, conversations, searchContacts, setActiveConversation } = useChat();
  const [search, setSearch] = useState("");

  const filtered = search.trim() ? searchContacts(search) : contacts;

  // 按在线状态排序
  const sorted = [...filtered].sort((a, b) => {
    const order: Record<string, number> = { online: 0, away: 1, busy: 2, offline: 3 };
    return (order[a.status] ?? 4) - (order[b.status] ?? 4);
  });

  const handleStartChat = (contact: Contact) => {
    // 查找已存在的私聊会话
    const existing = conversations.find(
      (c) =>
        c.type === "private" &&
        c.members.some((m) => m.userId === contact.userId)
    );
    if (existing) {
      setActiveConversation(existing.conversationId);
    }
  };

  return (
    <div>
      {/* 搜索 */}
      <div className="px-3 py-2">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="搜索联系人..."
          className="w-full px-3 py-1.5 text-sm bg-gray-100 rounded-lg
            placeholder:text-gray-400 focus:outline-none focus:ring-1 focus:ring-indigo-300"
        />
      </div>

      {/* 联系人列表 */}
      {sorted.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
          <UserPlus className="w-8 h-8 mb-2 opacity-40" />
          <p>暂无联系人</p>
        </div>
      ) : (
        sorted.map((contact) => (
          <button
            key={contact.userId}
            onClick={() => handleStartChat(contact)}
            className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 transition-colors text-left"
          >
            <div className="relative flex-shrink-0">
              <UserAvatar name={contact.displayName} size="md" />
              <OnlineBadge
                status={contact.status}
                size="sm"
                className="absolute -bottom-0.5 -right-0.5"
              />
            </div>
            <div className="flex-1 min-w-0">
              <h4 className="text-sm font-medium text-gray-900 truncate">
                {contact.displayName}
              </h4>
              <p className="text-xs text-gray-400">
                @{contact.username} · {getStatusText(contact.status)}
              </p>
            </div>
            <MessageSquare className="w-4 h-4 text-gray-300 group-hover:text-indigo-400 flex-shrink-0" />
          </button>
        ))
      )}
    </div>
  );
}
