"use client";

import React from "react";
import { Bell, LogOut, MessageCircle, UserRound, UsersRound } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import { ThemeToggle } from "../shared/ThemeToggle";

export type NavSection = "chats" | "friends" | "groups";

interface SidebarNavProps {
  activeSection: NavSection;
  onNavigate: (section: NavSection) => void;
  onOpenProfile: () => void;
}

export default function SidebarNav({ activeSection, onNavigate, onOpenProfile }: SidebarNavProps) {
  const { user, logout } = useAuth();
  const { friendRequestBadge, conversations } = useChat();
  const unread = conversations.reduce((sum, item) => sum + item.unreadCount, 0);
  const items = [
    { id: "chats" as const, label: "消息", icon: MessageCircle, badge: unread },
    { id: "friends" as const, label: "通讯录", icon: UserRound, badge: friendRequestBadge },
    { id: "groups" as const, label: "群组", icon: UsersRound, badge: 0 },
  ];

  return (
    <aside className="z-30 flex h-16 w-full flex-shrink-0 items-center border-t border-white/10 bg-rail px-3 text-zinc-400 md:h-full md:w-[72px] md:flex-col md:border-r md:border-t-0 md:border-white/10 md:px-0 md:py-4">
      <button
        onClick={onOpenProfile}
        className="group relative hidden h-10 w-10 items-center justify-center rounded-control md:flex"
        title="个人信息"
      >
        <UserAvatar src={user?.avatar} name={user?.displayName || "我"} size="md" className="rounded-control ring-1 ring-white/15" />
        <span className="absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-rail bg-accent" />
      </button>

      <nav className="flex flex-1 items-center justify-around md:mt-5 md:w-full md:flex-none md:flex-col md:gap-1.5">
        {items.map((item) => (
          <button
            key={item.id}
            onClick={() => onNavigate(item.id)}
            className={cn(
              "ui-press relative flex h-14 min-w-[64px] flex-col items-center justify-center gap-1 rounded-control text-[11px] transition duration-ui md:h-14 md:w-[56px] md:min-w-0",
              activeSection === item.id
                ? "bg-white/10 text-white"
                : "text-zinc-500 hover:bg-white/5 hover:text-zinc-200"
            )}
            title={item.label}
          >
            {activeSection === item.id && (
              <span className="absolute left-0 hidden h-5 w-[3px] rounded-r bg-accent md:block" />
            )}
            <item.icon className="h-5 w-5" strokeWidth={1.75} />
            <span>{item.label}</span>
            {item.badge > 0 && (
              <span className="absolute right-2 top-1 min-w-[17px] rounded-full bg-danger px-1 text-center text-[10px] font-semibold leading-[17px] text-white md:right-1">
                {item.badge > 99 ? "99+" : item.badge}
              </span>
            )}
          </button>
        ))}
      </nav>

      <button
        onClick={onOpenProfile}
        className="relative ml-1 flex h-11 w-11 items-center justify-center md:hidden"
        title="个人信息"
      >
        <UserAvatar src={user?.avatar} name={user?.displayName || "我"} size="sm" className="rounded-control ring-1 ring-white/15" />
        <span className="absolute bottom-1 right-1 h-2.5 w-2.5 rounded-full border-2 border-rail bg-accent" />
      </button>

      <div className="hidden flex-1 md:block" />
      <div className="hidden flex-col items-center gap-2 md:flex">
        <ThemeToggle compact />
        <button
          className="relative flex h-9 w-9 items-center justify-center rounded-control text-zinc-500 hover:bg-white/5 hover:text-zinc-200"
          title="通知中心"
        >
          <Bell className="h-4 w-4" strokeWidth={1.75} />
          <span className="absolute right-2 top-2 h-1.5 w-1.5 rounded-full bg-danger" />
        </button>
        <div className="my-1 h-px w-8 bg-white/10" />
        <button
          onClick={logout}
          className="flex h-9 w-9 items-center justify-center rounded-control text-zinc-500 hover:bg-danger/10 hover:text-danger"
          title="退出登录"
        >
          <LogOut className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>
    </aside>
  );
}
