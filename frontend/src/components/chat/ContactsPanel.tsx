"use client";

// ============================================================
// ContactsPanel — 联系人面板（含添加好友二级菜单）
// ============================================================
import React, { useState } from "react";
import { UserPlus, MessageSquare, Search, ArrowLeft } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import type { Contact } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { getStatusText } from "@/data/mock";
import AddFriendPanel from "./AddFriendPanel";

export default function ContactsPanel() {
  const { contacts, conversations, searchContacts, setActiveConversation } = useChat();
  const [search, setSearch] = useState("");
  const [showAddFriend, setShowAddFriend] = useState(false);

  // ---- 添加好友子页面 ----
  if (showAddFriend) {
    return (
      <div className="h-full flex flex-col bg-white w-full">
        <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-100 flex-shrink-0">
          <button
            onClick={() => setShowAddFriend(false)}
            className="p-1.5 -ml-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <h2 className="text-sm font-semibold text-gray-900">添加好友</h2>
        </div>
        <div className="flex-1 overflow-y-auto">
          <AddFriendPanel embedded />
        </div>
      </div>
    );
  }

  // ---- 联系人列表 ----
  const filtered = search.trim() ? searchContacts(search) : contacts;
  const sorted = [...filtered].sort((a, b) => {
    const order: Record<string, number> = { online: 0, away: 1, busy: 2, offline: 3 };
    return (order[a.status] ?? 4) - (order[b.status] ?? 4);
  });

  const handleStartChat = (contact: Contact) => {
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
    <div className="h-full flex flex-col bg-white w-full">
      {/* 头部 */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-gray-100 flex-shrink-0">
        <h2 className="text-sm font-semibold text-gray-900">联系人</h2>
        <button
          onClick={() => setShowAddFriend(true)}
          className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-indigo-600
            bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors"
        >
          <UserPlus className="w-3.5 h-3.5" />
          加好友
        </button>
      </div>

      {/* 搜索 */}
      <div className="px-3 py-2.5">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索联系人..."
            className="w-full pl-8 pr-3 py-1.5 text-xs bg-gray-100 rounded-lg
              placeholder:text-gray-400 focus:outline-none focus:ring-1 focus:ring-indigo-300"
          />
        </div>
      </div>

      {/* 联系人列表 */}
      <div className="flex-1 overflow-y-auto">
        {sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
            <UserPlus className="w-8 h-8 mb-2 opacity-40" />
            <p>暂无联系人</p>
            <p className="text-xs mt-1">点击上方"加好友"按钮添加</p>
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
    </div>
  );
}
