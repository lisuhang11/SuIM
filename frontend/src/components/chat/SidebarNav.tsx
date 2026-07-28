"use client";

import React from "react";
import { Bell, LogOut, MessageCircle, Settings, UserRound, UsersRound } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

export type NavSection = "chats" | "friends" | "groups";

interface SidebarNavProps {
  activeSection: NavSection;
  onNavigate: (section: NavSection) => void;
}

export default function SidebarNav({ activeSection, onNavigate }: SidebarNavProps) {
  const { user, logout } = useAuth();
  const { friendRequestBadge, conversations } = useChat();
  const unread = conversations.reduce((sum, item) => sum + item.unreadCount, 0);
  const items = [
    { id: "chats" as const, label: "消息", icon: MessageCircle, badge: unread },
    { id: "friends" as const, label: "通讯录", icon: UserRound, badge: friendRequestBadge },
    { id: "groups" as const, label: "群组", icon: UsersRound, badge: 1 },
  ];

  return (
    <aside className="z-30 flex h-16 w-full flex-shrink-0 items-center border-t border-slate-800 bg-[#172033] px-3 text-slate-300 md:h-full md:w-[76px] md:flex-col md:border-r md:border-t-0 md:px-0 md:py-4">
      <button
        onClick={() => onNavigate("chats")}
        className="hidden h-11 w-11 items-center justify-center rounded-lg bg-emerald-400 text-lg font-bold text-[#172033] md:flex"
        title="SuIM"
      >
        S
      </button>

      <nav className="flex flex-1 items-center justify-around md:mt-7 md:w-full md:flex-none md:flex-col md:gap-2">
        {items.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className={cn(
              "relative flex h-14 min-w-[64px] flex-col items-center justify-center gap-1 rounded-md text-[11px] transition-colors md:h-14 md:w-[60px] md:min-w-0",
              activeSection === item.id
                ? "bg-white/10 text-white"
                : "text-slate-400 hover:bg-white/5 hover:text-white"
            )}
            title={item.label}
          >
            {activeSection === item.id && <span className="absolute left-0 hidden h-6 w-[3px] rounded-r bg-emerald-400 md:block" />}
            <item.icon className="h-5 w-5" />
            <span>{item.label}</span>
            {item.badge > 0 && (
              <span className="absolute right-2 top-1 min-w-[17px] rounded-full bg-rose-500 px-1 text-center text-[10px] font-semibold leading-[17px] text-white md:right-1">
                {item.badge > 99 ? "99+" : item.badge}
              </span>
            )}
          </button>
        ))}
      </nav>

      <div className="hidden flex-1 md:block" />
      <div className="hidden flex-col items-center gap-2 md:flex">
        <button className="relative flex h-10 w-10 items-center justify-center rounded-md text-slate-400 hover:bg-white/5 hover:text-white" title="通知中心">
          <Bell className="h-5 w-5" />
          <span className="absolute right-2 top-2 h-1.5 w-1.5 rounded-full bg-rose-500" />
        </button>
        <button className="flex h-10 w-10 items-center justify-center rounded-md text-slate-400 hover:bg-white/5 hover:text-white" title="设置">
          <Settings className="h-5 w-5" />
        </button>
        <div className="my-1 h-px w-8 bg-white/10" />
        <div className="group relative">
          <UserAvatar src={user?.avatar} name={user?.displayName || "我"} size="md" className="ring-2 ring-white/20" />
          <span className="absolute bottom-0 right-0 h-3 w-3 rounded-full border-2 border-[#172033] bg-emerald-400" />
        </div>
        <button onClick={logout} className="flex h-9 w-9 items-center justify-center rounded-md text-slate-500 hover:bg-rose-500/10 hover:text-rose-400" title="退出登录">
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </aside>
  );
}
