"use client";

// ============================================================
// SidebarNav — 纵向图标导航栏
// 三个栏目：会话 | 好友 | 群聊
// ============================================================
import React, { useEffect, useState } from "react";
import {
  MessageSquare,
  Users,
  UserPlus,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import { wsManager } from "@/services/websocket";

export type NavSection = "chats" | "friends" | "groups";

interface SidebarNavProps {
  activeSection: NavSection;
  onNavigate: (section: NavSection) => void;
}

export default function SidebarNav({ activeSection, onNavigate }: SidebarNavProps) {
  const { user, logout } = useAuth();
  const { friendRequestBadge } = useChat();
  const router = useRouter();
  const [groupBadge, setGroupBadge] = useState(0);
  const [wsConnected, setWsConnected] = useState(false);

  // 监听 WebSocket 连接状态
  useEffect(() => {
    const unsub = wsManager.onStatusChange(setWsConnected);
    return unsub;
  }, []);

  // 首次加载 + 定期拉取群 badge（群推送暂未实现，保留轮询作为过渡）
  useEffect(() => {
    const fetchGroupBadge = async () => {
      try {
        const api = await import("@/services/api");
        const count = await api.getUnhandledGroupApplicationCount();
        setGroupBadge(count);
      } catch { /* ignore */ }
    };
    fetchGroupBadge();
    const timer = setInterval(fetchGroupBadge, 30000);
    return () => clearInterval(timer);
  }, []);

  const navItems: {
    id: NavSection;
    icon: React.ElementType;
    label: string;
    badge?: number;
  }[] = [
    { id: "chats", icon: MessageSquare, label: "会话" },
    { id: "friends", icon: Users, label: "好友", badge: friendRequestBadge },
    { id: "groups", icon: UserPlus, label: "群聊", badge: groupBadge },
  ];

  const handleLogout = async () => {
    await logout();
    router.push("/login");
  };

  return (
    <div className="h-full w-16 flex flex-col items-center bg-gray-900 text-gray-300 py-3 gap-1 flex-shrink-0 select-none">
      {/* 用户头像 + WS 状态指示 */}
      <button
        onClick={() => onNavigate("chats")}
        className="mb-2 relative"
        title={user?.displayName || user?.username || "用户"}
      >
        <UserAvatar
          name={user?.displayName || user?.username || ""}
          size="sm"
          className="ring-2 ring-gray-700 hover:ring-indigo-400 transition-all"
        />
        {/* WebSocket 连接状态指示器 */}
        <span
          className={cn(
            "absolute -bottom-0.5 -right-0.5 w-3 h-3 rounded-full border-2 border-gray-900 transition-colors",
            wsConnected ? "bg-green-400" : "bg-red-400"
          )}
          title={wsConnected ? "实时连接正常" : "实时连接断开"}
        />
      </button>

      {/* 分割线 */}
      <div className="w-8 h-px bg-gray-700 my-1" />

      {/* 导航图标 */}
      {navItems.map((item) => (
        <button
          key={item.id}
          onClick={() => onNavigate(item.id)}
          className={cn(
            "relative w-11 h-11 flex items-center justify-center rounded-xl transition-all duration-200 group",
            activeSection === item.id
              ? "bg-indigo-500 text-white shadow-lg shadow-indigo-500/30"
              : "text-gray-400 hover:text-gray-200 hover:bg-gray-800"
          )}
          title={item.label}
        >
          <item.icon className="w-5 h-5" />
          {/* 激活指示条 */}
          {activeSection === item.id && (
            <div className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-white rounded-r-full" />
          )}
          {/* 徽章 */}
          {item.badge !== undefined && item.badge > 0 && (
            <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 flex items-center justify-center
              px-1 text-[10px] font-bold text-white bg-red-500 rounded-full ring-2 ring-gray-900">
              {item.badge > 99 ? "99+" : item.badge}
            </span>
          )}
          {/* 悬停提示 */}
          <div className="absolute left-full ml-3 px-2 py-1 bg-gray-800 text-white text-xs rounded-md
            opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity whitespace-nowrap z-50">
            {item.label}
          </div>
        </button>
      ))}

      {/* 底部操作 — 退出登录 */}
      <div className="mt-auto flex flex-col items-center gap-1 pb-2">
        <button
          onClick={handleLogout}
          className="w-11 h-11 flex items-center justify-center rounded-xl
            text-gray-300 hover:text-red-400 hover:bg-red-500/20 transition-all duration-200 group relative"
          title="退出登录"
        >
          <LogOut className="w-5 h-5" />
          {/* 悬停提示 */}
          <div className="absolute left-full ml-3 px-2 py-1 bg-red-600 text-white text-xs rounded-md
            opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity whitespace-nowrap z-50">
            退出登录
          </div>
        </button>
      </div>
    </div>
  );
}
