"use client";

// ============================================================
// ConversationList — 会话列表（垂直侧边导航栏）
// 侧边栏带滑动紫色指示器动画
// ============================================================
import React, { useState, useMemo, useRef, useEffect, useCallback } from "react";
import {
  MessageSquare,
  ContactRound,
  LogOut,
  Settings,
  Bell,
  UsersRound,
  Mail,
  Circle,
  Search,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import ConversationItem from "./ConversationItem";
import CreateGroupDialog from "./CreateGroupDialog";
import ContactsPanel from "./ContactsPanel";
import GroupsPanel from "./GroupsPanel";
import NotificationsPanel from "./NotificationsPanel";

type TabType = "chats" | "contacts" | "groups" | "notifications";

const SEARCH_COLLAPSE_W = 170;

export default function ConversationList({ panelWidth = 320 }: { panelWidth?: number }) {
  const { user, logout } = useAuth();
  const {
    conversations,
    activeConversationId,
    setActiveConversation,
    wsConnected,
    unreadNotificationCount,
    friendRequests,
  } = useChat();
  const router = useRouter();

  const [search, setSearch] = useState("");
  const [searchFocused, setSearchFocused] = useState(false);
  const [tab, setTab] = useState<TabType>("chats");
  const isNarrow = panelWidth < SEARCH_COLLAPSE_W;
  const [groupPanelOpen, setGroupPanelOpen] = useState(false);
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [showProfile, setShowProfile] = useState(false);
  const profileTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const avatarBtnRef = useRef<HTMLButtonElement>(null);
  const avatarSlotRef = useRef<HTMLDivElement>(null);
  const avatarAnchorRef = useRef<HTMLDivElement>(null);
  const profileContainerRef = useRef<HTMLDivElement>(null);
  const [avatarOrigin, setAvatarOrigin] = useState({ top: 0, left: 0, w: 32, h: 32 });
  const [avatarTarget, setAvatarTarget] = useState({ top: 0, left: 0, w: 56, h: 56 });

  // ---- 滑动指示器状态 ----
  const sidebarRef = useRef<HTMLDivElement>(null);
  const btnRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const [indicatorStyle, setIndicatorStyle] = useState<React.CSSProperties>({
    top: 0, height: 0, opacity: 0,
  });
  const [isTransitioning, setIsTransitioning] = useState(false);
  const transitionTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const updateIndicator = useCallback(() => {
    const btn = btnRefs.current[tab];
    const sidebar = sidebarRef.current;
    if (!btn || !sidebar) return;
    const sidebarRect = sidebar.getBoundingClientRect();
    const btnRect = btn.getBoundingClientRect();
    setIndicatorStyle({
      top: btnRect.top - sidebarRect.top,
      height: btnRect.height,
      opacity: 1,
    });
  }, [tab]);

  useEffect(() => {
    const raf = requestAnimationFrame(updateIndicator);
    return () => cancelAnimationFrame(raf);
  }, [updateIndicator, conversations]);

  useEffect(() => {
    updateIndicator();
    // 抑制 hover 背景 320ms（比 CSS transition 300ms 略长）
    setIsTransitioning(true);
    if (transitionTimerRef.current) clearTimeout(transitionTimerRef.current);
    transitionTimerRef.current = setTimeout(() => setIsTransitioning(false), 320);
    return () => {
      if (transitionTimerRef.current) clearTimeout(transitionTimerRef.current);
    };
  }, [tab, updateIndicator]);

  const handleTabClick = useCallback((key: TabType) => {
    if (key === "groups" && tab === "groups") {
      // 已在群聊页，切换面板开合
      setGroupPanelOpen((p) => !p);
      return;
    }
    if (key === tab) return;
    setTab(key);
    if (key === "groups") setGroupPanelOpen(false);
  }, [tab]);

  const handleProfileEnter = () => {
    if (profileTimerRef.current) clearTimeout(profileTimerRef.current);
    setShowProfile(true);
    // 静态计算槽位坐标：不依赖过渡中的 DOM，瞬间精准
    if (avatarBtnRef.current && avatarAnchorRef.current) {
      const from = avatarBtnRef.current.getBoundingClientRect();
      const anchor = avatarAnchorRef.current.getBoundingClientRect();
      setAvatarOrigin({ top: from.top, left: from.left, w: from.width, h: from.height });
      // 槽位在弹出窗内：popup left-[72px] + slot left-4(16px) = 距锚点左缘 88px
      //                  popup top-8(32px) + slot -top-7(-28px) = 距锚点顶缘 4px
      const slotLeft = anchor.left + 88;
      const slotTop = anchor.top + 4;
      const targetW = 56, targetH = 56;
      setAvatarTarget({ top: slotTop, left: slotLeft + 56 - targetW, w: targetW, h: targetH });
    }
  };
  const handleProfileLeave = () => {
    profileTimerRef.current = setTimeout(() => setShowProfile(false), 500);
  };

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

  const handleLogout = async () => {
    await logout();
    router.push("/login");
  };

  const tabs: { key: TabType; icon: React.ReactNode; label: string; badge?: number }[] = [
    {
      key: "chats",
      icon: <MessageSquare className="w-5 h-5" />,
      label: "聊天",
    },
    {
      key: "contacts",
      icon: <ContactRound className="w-[21px] h-[21px]" />,
      label: "联系人",
    },
    {
      key: "groups",
      icon: <UsersRound className="w-5 h-5" />,
      label: "群聊",
    },
    {
      key: "notifications",
      icon: <Bell className="w-5 h-5" />,
      label: "通知",
      badge: unreadNotificationCount,
    },
  ];

  return (
    <div className="h-full flex bg-white">
      {/* ====== 左侧垂直导航栏 ====== */}
      <div
        ref={sidebarRef}
        className="w-[68px] flex-shrink-0 flex flex-col items-center py-3 gap-1
          bg-gray-50 border-r border-gray-100 relative z-[60] shadow-[2px_0_10px_rgba(0,0,0,0.06)]"
      >
        {/* ---- 滑动紫色指示器 ---- */}
        <div
          className={cn(
            "absolute left-1.5 right-1.5 rounded-xl bg-indigo-500 shadow-md shadow-indigo-200",
            "transition-all duration-300 ease-[cubic-bezier(0.34,1.56,0.64,1)]",
            "pointer-events-none z-0"
          )}
          style={{
            top: `${indicatorStyle.top}px`,
            height: `${indicatorStyle.height}px`,
            opacity: indicatorStyle.opacity,
            transform: indicatorStyle.opacity ? "scale(1)" : "scale(0.8)",
          }}
        />

        {/* 头像 — hover 飞入弹出窗 */}
        <div
          ref={profileContainerRef}
          className="relative mb-3 z-10"
          onMouseEnter={handleProfileEnter}
          onMouseLeave={handleProfileLeave}
        >
          {/* 定位锚 — 弹出窗以此为参照 */}
          <div ref={avatarAnchorRef} className="relative">
            {/* 原位头像 — hover 时缩隐 */}
            <button
              ref={avatarBtnRef}
            className={cn(
              "relative transition-all duration-300 ease-out",
              showProfile && "opacity-0 scale-50"
            )}
          >
            <UserAvatar
              src={user?.avatar}
              name={user?.displayName || user?.username || ""}
              size="sm"
            />
            <span className={cn(
              "absolute -bottom-0.5 -right-0.5 w-2.5 h-2.5 rounded-full border-2 border-gray-50 transition-opacity duration-300",
              showProfile ? "opacity-0" : "",
              wsConnected ? "bg-green-500" : "bg-red-400"
            )} />
          </button>

          {/* 浮空头像 — 从侧栏位置飞入弹出窗 */}
          <div
            className={cn(
              "fixed z-[70] rounded-full overflow-hidden transition-all duration-250 ease-out",
              "shadow-lg pointer-events-none"
            )}
            style={{
              top: showProfile ? avatarTarget.top : avatarOrigin.top,
              left: showProfile ? avatarTarget.left : avatarOrigin.left,
              width: showProfile ? avatarTarget.w : avatarOrigin.w,
              height: showProfile ? avatarTarget.h : avatarOrigin.h,
              opacity: showProfile ? 1 : 0,
            }}
          >
            <UserAvatar
              src={user?.avatar}
              name={user?.displayName || user?.username || ""}
              size="xl"
              className="w-full h-full"
            />
          </div>

          {/* 个人信息弹出卡片 — 向右滑出，含头像槽位 */}
          <div
            className={cn(
              "absolute left-[72px] top-8 z-50 transition-all duration-200 ease-out",
              showProfile
                ? "opacity-100 translate-x-0"
                : "opacity-0 -translate-x-3 pointer-events-none"
            )}
            onMouseEnter={handleProfileEnter}
            onMouseLeave={handleProfileLeave}
          >
            <div className="w-56 bg-white rounded-2xl shadow-xl border border-gray-100 p-4 pt-10 relative">
              {/* 头像槽位 — 飞入头像的目标位置 */}
              <div
                ref={avatarSlotRef}
                className="absolute -top-7 left-4 w-14 h-14 rounded-full bg-gray-100 ring-4 ring-white"
              />
              {/* 文字 */}
              <div>
                <h3 className="font-semibold text-gray-900 text-sm truncate">
                  {user?.displayName || user?.username}
                </h3>
                <p className="text-xs text-gray-400">@{user?.username}</p>
              </div>
              <div className="space-y-2 text-sm">
                <div className="flex items-center gap-2 text-gray-500">
                  <span className="text-xs text-gray-400 w-4 text-center font-mono">ID</span>
                  <span className="truncate text-xs font-mono text-gray-600">{user?.suid || "-"}</span>
                </div>
                <div className="flex items-center gap-2 text-gray-500">
                  <Mail className="w-4 h-4 text-gray-400" />
                  <span className="truncate text-xs">{user?.email || "未设置邮箱"}</span>
                </div>
                <div className="flex items-center gap-2">
                  <Circle className={cn("w-3 h-3 fill-current",
                    wsConnected ? "text-green-500" : "text-red-400")} />
                  <span className="text-xs text-gray-500">
                    {wsConnected ? "在线" : "离线"}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
        </div>

        {/* 导航按钮 */}
        {tabs.map((t) => (
          <button
            key={t.key}
            ref={(el) => { btnRefs.current[t.key] = el; }}
            onClick={() => handleTabClick(t.key)}
            className={cn(
              "relative w-11 h-11 rounded-xl flex items-center justify-center z-10 shadow-none",
              "transition-colors duration-200",
              tab === t.key
                ? "text-white"
                : cn(
                    "text-gray-400 hover:text-gray-600",
                    !isTransitioning && "hover:bg-gray-200/60"
                  )
            )}
            title={t.label}
          >
            {t.icon}
            {t.badge != null && t.badge > 0 && (
              <span className="absolute -top-0.5 -right-0.5 bg-red-500 text-white text-[10px]
                font-bold min-w-[16px] h-[16px] rounded-full flex items-center justify-center px-1
                shadow-sm z-20">
                {t.badge > 9 ? "9+" : t.badge}
              </span>
            )}
          </button>
        ))}

        <div className="flex-1" />

        {/* 底部操作按钮 */}
        <button
          onClick={() => router.push("/settings")}
          className="w-11 h-11 rounded-xl flex items-center justify-center z-10
            text-gray-400 hover:text-gray-600 hover:bg-gray-200/60 transition-all"
          title="设置"
        >
          <Settings className="w-5 h-5" />
        </button>
        <button
          onClick={handleLogout}
          className="w-11 h-11 rounded-xl flex items-center justify-center z-10
            text-gray-400 hover:text-red-500 hover:bg-red-50 transition-all"
          title="退出登录"
        >
          <LogOut className="w-4 h-4" />
        </button>
      </div>

      {/* ====== 右侧主内容区 ====== */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* 搜索框 — 窄面板缩为图标；<340px 聚焦强制弹340px */}
        {tab !== "notifications" && (
          <div className="px-4 py-3 relative">
            {isNarrow && !searchFocused ? (
              <button
                onClick={() => setSearchFocused(true)}
                className="w-9 h-9 flex items-center justify-center rounded-xl bg-gray-100 text-gray-400 hover:bg-gray-200 transition-all"
              >
                <Search className="w-4 h-4" />
              </button>
            ) : (
              <div className={cn(
                panelWidth < 340 && searchFocused && "absolute left-4 top-3 z-20"
              )} style={panelWidth < 340 && searchFocused ? { width: 340 } : undefined}>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
                  <input
                    type="text"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    onFocus={() => setSearchFocused(true)}
                    onBlur={() => { if (!search) setSearchFocused(false); }}
                    autoFocus={isNarrow}
                    placeholder={tab === "chats" ? "搜索会话..." : "搜索联系人..."}
                    className={cn(
                      "w-full pl-9 pr-4 py-2 rounded-xl text-sm outline-none transition-all",
                      "placeholder:text-gray-400 focus:ring-2 focus:ring-indigo-100 focus:border-indigo-400",
                      panelWidth < 340 && searchFocused
                        ? "bg-white border border-gray-200 shadow-lg"
                        : "bg-gray-100 border border-transparent"
                    )}
                  />
                </div>
              </div>
            )}
          </div>
        )}

        {/* 内容 */}
        <div className="flex-1 overflow-y-auto">
          {tab === "notifications" ? (
            <NotificationsPanel onClose={() => setTab("chats")} />
          ) : tab === "chats" ? (
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
          ) : tab === "groups" ? (
            <GroupsPanel panelOpen={groupPanelOpen} onPanelToggle={setGroupPanelOpen} />
          ) : (
            <ContactsPanel
              pendingRequestCount={(friendRequests || []).filter((r) => r.status === "pending").length}
            />
          )}
        </div>
      </div>

      {showCreateGroup && (
        <CreateGroupDialog onClose={() => setShowCreateGroup(false)} />
      )}
    </div>
  );
}
