"use client";

// ============================================================
// SidebarNav — 纵向图标导航栏
// 三个栏目：会话 | 好友 | 群聊
// ============================================================
import React, { useEffect, useState, useCallback } from "react";
import {
  MessageSquare,
  Users,
  UserPlus,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

export type NavSection = "chats" | "friends" | "groups";

interface SidebarNavProps {
  activeSection: NavSection;
  onNavigate: (section: NavSection) => void;
}

export default function SidebarNav({ activeSection, onNavigate }: SidebarNavProps) {
  const { user, logout } = useAuth();
  const router = useRouter();
  const [friendBadge, setFriendBadge] = useState(0);
  const [groupBadge, setGroupBadge] = useState(0);

  // 定期拉取未处理的好友请求数 & 入群申请数
  const fetchBadges = useCallback(async () => {
    try {
      const api = await import("@/services/api");
      const [friendCount, groupCount] = await Promise.allSettled([
        api.getUnhandledRequestCount(),
        api.getUnhandledGroupApplicationCount(),
      ]);
      if (friendCount.status === "fulfilled") setFriendBadge(friendCount.value);
      if (groupCount.status === "fulfilled") setGroupBadge(groupCount.value);
    } catch {
      // 忽略，使用 mock badge
    }
  }, []);

  useEffect(() => {
    fetchBadges();
    const timer = setInterval(fetchBadges, 30000); // 30s 轮询
    return () => clearInterval(timer);
  }, [fetchBadges]);

  const navItems: {
    id: NavSection;
    icon: React.ElementType;
    label: string;
    badge?: number;
  }[] = [
    { id: "chats", icon: MessageSquare, label: "会话" },
    { id: "friends", icon: Users, label: "好友", badge: friendBadge },
    { id: "groups", icon: UserPlus, label: "群聊", badge: groupBadge },
  ];

  const handleLogout = async () => {
    await logout();
    router.push("/login");
  };

  return (
    <div className="h-full w-16 flex flex-col items-center bg-gray-900 text-gray-300 py-3 gap-1 flex-shrink-0 select-none">
      {/* 用户头像 */}
      <button
        onClick={() => onNavigate("chats")}
        className="mb-2"
        title={user?.displayName || user?.username || "用户"}
      >
        <UserAvatar
          name={user?.displayName || user?.username || ""}
          size="sm"
          className="ring-2 ring-gray-700 hover:ring-indigo-400 transition-all"
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

      {/* 底部操作 */}
      <div className="mt-auto flex flex-col items-center gap-1 pb-2">
        <button
          onClick={handleLogout}
          className="w-11 h-11 flex items-center justify-center rounded-xl
            text-gray-500 hover:text-red-400 hover:bg-gray-800 transition-all duration-200 group"
          title="退出登录"
        >
          <LogOut className="w-5 h-5" />
          <div className="absolute left-full ml-3 px-2 py-1 bg-gray-800 text-white text-xs rounded-md
            opacity-0 group-hover:opacity-100 pointer-events-none transition-opacity whitespace-nowrap z-50">
            退出登录
          </div>
        </button>
      </div>
    </div>
  );
}
