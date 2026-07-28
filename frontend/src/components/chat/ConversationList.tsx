"use client";

// ============================================================
// ConversationList — 会话列表（图标导航下的聊天列表面板）
// ============================================================
import React, { useState, useMemo } from "react";
import {
  MessageSquare,
  UsersRound,
  Search,
  Wifi,
  WifiOff,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import ConversationItem from "./ConversationItem";
import CreateGroupDialog from "./CreateGroupDialog";
import UserAvatar from "../shared/UserAvatar";

export default function ConversationList() {
  const { user } = useAuth();
  const {
    conversations,
    activeConversationId,
    setActiveConversation,
    wsConnected,
  } = useChat();

  const [search, setSearch] = useState("");
  const [showCreateGroup, setShowCreateGroup] = useState(false);

  const filtered = useMemo(() => {
    if (!search.trim()) return conversations;
    const q = search.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, search]);

  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => {
      if (a.isPinned && !b.isPinned) return -1;
      if (!a.isPinned && b.isPinned) return 1;
      return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
    });
  }, [filtered]);

  return (
    <div className="h-full flex flex-col bg-white w-full">
      {/* 头部: 标题 + 用户信息 */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-gray-100 flex-shrink-0">
        <div className="flex items-center gap-2.5">
          <UserAvatar
            name={user?.displayName || user?.username || ""}
            size="sm"
          />
          <div>
            <h2 className="text-sm font-semibold text-gray-900 truncate max-w-[120px]">
              {user?.displayName || user?.username}
            </h2>
            <div className="flex items-center gap-1 text-[10px] text-gray-400">
              <span className={wsConnected ? "text-green-500" : "text-red-400"}>
                {wsConnected ? "●" : "○"}
              </span>
              {wsConnected ? "在线" : "离线"}
            </div>
          </div>
        </div>
        <button
          onClick={() => setShowCreateGroup(true)}
          className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-indigo-500 transition-colors"
          title="创建群聊"
        >
          <UsersRound className="w-4 h-4" />
        </button>
      </div>

      {/* 搜索框 */}
      <div className="px-3 py-2.5">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索会话..."
            className="w-full pl-8 pr-3 py-1.5 text-xs bg-gray-100 rounded-lg
              placeholder:text-gray-400 focus:outline-none focus:ring-1 focus:ring-indigo-300"
          />
        </div>
      </div>

      {/* 会话列表 */}
      <div className="flex-1 overflow-y-auto">
        {sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-400 text-sm px-4">
            <MessageSquare className="w-8 h-8 mb-2 opacity-40" />
            <p>暂无会话</p>
            <p className="text-xs mt-1">创建群聊开始对话</p>
          </div>
        ) : (
          sorted.map((conv) => (
            <ConversationItem
              key={conv.conversationId}
              conversation={conv}
              isActive={conv.conversationId === activeConversationId}
              onClick={() => setActiveConversation(conv.conversationId)}
            />
          ))
        )}
      </div>

      {showCreateGroup && (
        <CreateGroupDialog onClose={() => setShowCreateGroup(false)} />
      )}
    </div>
  );
}
