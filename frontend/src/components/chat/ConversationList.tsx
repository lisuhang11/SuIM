"use client";

// ============================================================
// ConversationList — 会话列表侧边栏
// ============================================================
import React, { useState, useMemo } from "react";
import {
  MessageSquare,
  Users,
  Plus,
  UserPlus,
  UsersRound,
  LogOut,
  Settings,
  Wifi,
  WifiOff,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import SearchInput from "../shared/SearchInput";
import ConversationItem from "./ConversationItem";
import CreateGroupDialog from "./CreateGroupDialog";
import ContactsPanel from "./ContactsPanel";

export default function ConversationList() {
  const { user, logout } = useAuth();
  const {
    conversations,
    activeConversationId,
    setActiveConversation,
    wsConnected,
  } = useChat();
  const router = useRouter();

  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<"chats" | "contacts">("chats");
  const [showCreateGroup, setShowCreateGroup] = useState(false);

  // 过滤会话
  const filtered = useMemo(() => {
    if (!search.trim()) return conversations;
    const q = search.toLowerCase();
    return conversations.filter((c) => c.title.toLowerCase().includes(q));
  }, [conversations, search]);

  // 排序：置顶优先，按最后消息时间
  const sorted = useMemo(() => {
    return [...filtered].sort((a, b) => {
      if (a.isPinned && !b.isPinned) return -1;
      if (!a.isPinned && b.isPinned) return 1;
      return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
    });
  }, [filtered]);

  const handleLogout = async () => {
    await logout();
    router.push("/login");
  };

  return (
    <div className="h-full flex flex-col bg-white border-r border-gray-200">
      {/* 用户信息 */}
      <div className="h-16 flex items-center justify-between px-4 border-b border-gray-200 flex-shrink-0">
        <div className="flex items-center gap-3">
          <UserAvatar
            src={user?.avatar}
            name={user?.displayName || user?.username || ""}
            size="md"
          />
          <div className="min-w-0">
            <h2 className="font-semibold text-gray-900 text-sm truncate">
              {user?.displayName || user?.username}
            </h2>
            <div className="flex items-center gap-1 text-xs text-gray-400">
              {wsConnected ? (
                <Wifi className="w-3 h-3 text-green-500" />
              ) : (
                <WifiOff className="w-3 h-3 text-red-400" />
              )}
              <span>{wsConnected ? "已连接" : "未连接"}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setShowCreateGroup(true)}
            className="p-2 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            title="创建群聊"
          >
            <UsersRound className="w-4 h-4" />
          </button>
          <button
            onClick={() => router.push("/settings")}
            className="p-2 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            title="设置"
          >
            <Settings className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* 搜索框 */}
      <div className="px-3 py-3">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="搜索会话..."
        />
      </div>

      {/* 标签页切换 */}
      <div className="flex border-b border-gray-100 px-3">
        <button
          onClick={() => setTab("chats")}
          className={cn(
            "flex items-center gap-1.5 px-4 py-2 text-sm font-medium transition-colors border-b-2",
            tab === "chats"
              ? "text-indigo-500 border-indigo-500"
              : "text-gray-400 border-transparent hover:text-gray-600"
          )}
        >
          <MessageSquare className="w-4 h-4" />
          <span>聊天</span>
        </button>
        <button
          onClick={() => setTab("contacts")}
          className={cn(
            "flex items-center gap-1.5 px-4 py-2 text-sm font-medium transition-colors border-b-2",
            tab === "contacts"
              ? "text-indigo-500 border-indigo-500"
              : "text-gray-400 border-transparent hover:text-gray-600"
          )}
        >
          <Users className="w-4 h-4" />
          <span>联系人</span>
        </button>
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-y-auto">
        {tab === "chats" ? (
          <>
            {sorted.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full text-gray-400 text-sm px-4">
                <MessageSquare className="w-8 h-8 mb-2 opacity-40" />
                <p>暂无会话</p>
                <p className="text-xs mt-1">点击 + 开始新对话</p>
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
          </>
        ) : (
          <ContactsPanel />
        )}
      </div>

      {/* 底部操作 */}
      <div className="border-t border-gray-200 p-2 flex items-center">
        <button
          onClick={handleLogout}
          className="w-full flex items-center justify-center gap-2 py-2 rounded-xl
            text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors text-sm"
        >
          <LogOut className="w-4 h-4" />
          <span>退出登录</span>
        </button>
      </div>

      {/* 创建群聊对话框 */}
      {showCreateGroup && (
        <CreateGroupDialog onClose={() => setShowCreateGroup(false)} />
      )}
    </div>
  );
}
