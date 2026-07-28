"use client";

// ============================================================
// GroupsPanel — 群聊栏目（群列表 + 创建群聊 + 入群申请 + 群详情）
// ============================================================
import React, { useState, useEffect, useCallback } from "react";
import {
  Search,
  Plus,
  Bell,
  ArrowLeft,
  Users,
  Settings,
  UserPlus,
  UserX,
  LogOut,
  Shield,
  VolumeX,
  Volume2,
  Trash2,
  Loader2,
  Crown,
  Check,
  X,
  RefreshCw,
} from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { useAuth } from "@/contexts/AuthContext";
import type { Group, GroupMemberInfo, GroupApplication } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";

type SubView = "list" | "detail" | "applications";

export default function GroupsPanel() {
  const { groups, contacts, setActiveConversation, conversations, createGroup, refreshConversations } = useChat();
  const { user: currentUser } = useAuth();
  const [subView, setSubView] = useState<SubView>("list");
  const [selectedGroup, setSelectedGroup] = useState<Group | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  // ---- 群申请 badge ----
  const [applicationCount, setApplicationCount] = useState(0);
  const fetchBadge = useCallback(async () => {
    try {
      const api = await import("@/services/api");
      const count = await api.getUnhandledGroupApplicationCount();
      setApplicationCount(count);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    fetchBadge();
    const timer = setInterval(fetchBadge, 30000);
    return () => clearInterval(timer);
  }, [fetchBadge]);

  // ---- 搜索 ----
  const [search, setSearch] = useState("");
  const filtered = groups.filter(
    (g) => !search.trim() || g.name.toLowerCase().includes(search.toLowerCase())
  );

  const handleOpenChat = (group: Group) => {
    const existing = conversations.find(
      (c) => c.type === "group" && c.conversationId === group.groupId
    );
    if (existing) {
      setActiveConversation(existing.conversationId);
    }
  };

  const handleGroupClick = (group: Group) => {
    setSelectedGroup(group);
    setSubView("detail");
  };

  // ========== 群详情子视图 ==========
  if (subView === "detail" && selectedGroup) {
    return (
      <GroupDetailPanel
        group={selectedGroup}
        currentUser={currentUser}
        onBack={() => setSubView("list")}
        onOpenChat={handleOpenChat}
        onGroupUpdated={() => {
          refreshConversations();
        }}
      />
    );
  }

  // ========== 入群申请子视图 ==========
  if (subView === "applications") {
    return (
      <div className="h-full flex flex-col bg-white w-full">
        <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-100 flex-shrink-0">
          <button
            onClick={() => { setSubView("list"); fetchBadge(); }}
            className="p-1.5 -ml-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <h2 className="text-sm font-semibold text-gray-900">入群申请</h2>
        </div>
        <div className="flex-1 overflow-y-auto">
          <ApplicationsPanel groups={groups} onHandled={fetchBadge} />
        </div>
      </div>
    );
  }

  // ========== 默认：群列表 ==========
  return (
    <div className="h-full flex flex-col bg-white w-full">
      {/* 头部 */}
      <div className="h-14 flex items-center justify-between px-4 border-b border-gray-100 flex-shrink-0">
        <h2 className="text-sm font-semibold text-gray-900">群聊</h2>
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium text-indigo-600
              bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors"
          >
            <Plus className="w-3.5 h-3.5" />
            创建
          </button>
          <button
            onClick={() => setSubView("applications")}
            className="relative p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
            title="入群申请"
          >
            <Bell className="w-4 h-4" />
            {applicationCount > 0 && (
              <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 flex items-center justify-center
                px-1 text-[10px] font-bold text-white bg-red-500 rounded-full">
                {applicationCount > 99 ? "99+" : applicationCount}
              </span>
            )}
          </button>
        </div>
      </div>

      {/* 搜索 */}
      <div className="px-3 py-2.5">
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索群聊..."
            className="w-full pl-8 pr-3 py-1.5 text-xs bg-gray-100 rounded-lg
              placeholder:text-gray-400 focus:outline-none focus:ring-1 focus:ring-indigo-300"
          />
        </div>
      </div>

      {/* 群列表 */}
      <div className="flex-1 overflow-y-auto">
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
            <Users className="w-8 h-8 mb-2 opacity-40" />
            <p>暂无群聊</p>
            <p className="text-xs mt-1">点击"创建"按钮创建群聊</p>
          </div>
        ) : (
          filtered.map((group) => (
            <div
              key={group.groupId}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-50 transition-colors cursor-pointer group"
              onClick={() => handleOpenChat(group)}
            >
              {/* 群头像：使用首字母 */}
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-400 to-purple-500
                flex items-center justify-center text-white font-bold text-sm flex-shrink-0">
                {group.name.slice(0, 2)}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <h4 className="text-sm font-medium text-gray-900 truncate">{group.name}</h4>
                  {group.ownerId === currentUser?.userId && (
                    <span className="text-[10px] px-1.5 py-0.5 bg-amber-100 text-amber-700 rounded-full font-medium flex-shrink-0">
                      群主
                    </span>
                  )}
                </div>
                <p className="text-xs text-gray-400">{group.memberCount} 名成员</p>
              </div>
              {/* hover 时显示详情按钮 */}
              <button
                onClick={(e) => { e.stopPropagation(); handleGroupClick(group); }}
                className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600
                  opacity-0 group-hover:opacity-100 transition-opacity"
                title="群详情"
              >
                <Settings className="w-4 h-4" />
              </button>
            </div>
          ))
        )}
      </div>

      {/* 创建群聊对话框 */}
      {showCreate && (
        <CreateGroupDialogEmbedded
          contacts={contacts}
          currentUser={currentUser}
          onCreateGroup={async (name: string, memberIds: string[]) => {
            const conv = await createGroup(name, memberIds);
            if (conv) {
              setShowCreate(false);
              setActiveConversation(conv.conversationId);
            }
          }}
          onClose={() => setShowCreate(false)}
        />
      )}
    </div>
  );
}

// ============================================================
// 群详情面板
// ============================================================
function GroupDetailPanel({
  group,
  currentUser,
  onBack,
  onOpenChat,
  onGroupUpdated,
}: {
  group: Group;
  currentUser: any;
  onBack: () => void;
  onOpenChat: (g: Group) => void;
  onGroupUpdated: () => void;
}) {
  type DetailTab = "members" | "applications" | "settings";
  const [tab, setTab] = useState<DetailTab>("members");
  const [members, setMembers] = useState<GroupMemberInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [applications, setApplications] = useState<GroupApplication[]>([]);
  const [appCount, setAppCount] = useState(0);

  const loadMembers = useCallback(async () => {
    setLoading(true);
    try {
      const api = await import("@/services/api");
      const list = await api.getGroupMembers(group.groupId);
      setMembers(list);
    } catch {
      // API 不可用，保持空列表
    } finally {
      setLoading(false);
    }
  }, [group, currentUser]);

  const loadApplications = useCallback(async () => {
    try {
      const api = await import("@/services/api");
      const apps = await api.getPendingApplications(group.groupId);
      setApplications(apps.filter((a) => a.status === "pending"));
      setAppCount(apps.filter((a) => a.status === "pending").length);
    } catch { /* ignore */ }
  }, [group]);

  useEffect(() => {
    loadMembers();
    loadApplications();
  }, [loadMembers, loadApplications]);

  const isOwner = group.ownerId === currentUser?.userId;
  const currentMember = members.find((m) => m.userId === currentUser?.userId);
  const isAdmin = currentMember?.roleLevel === 1 || currentMember?.roleLevel === 2;
  const isOwnerOrAdmin = isOwner || isAdmin;

  const handleKickMember = async (userId: string) => {
    if (!window.confirm("确定要踢出该成员吗？")) return;
    try {
      const api = await import("@/services/api");
      await api.kickGroupMember(group.groupId, userId);
      setMembers((prev) => prev.filter((m) => m.userId !== userId));
    } catch { /* fallback */ }
  };

  const handleTransferOwner = async (userId: string) => {
    if (!window.confirm("确定要转让群主给该成员吗？此操作不可撤销。")) return;
    try {
      const api = await import("@/services/api");
      await api.transferGroupOwner(group.groupId, userId);
      onGroupUpdated();
    } catch { /* fallback */ }
  };

  const handleQuitGroup = async () => {
    if (isOwner) {
      if (!window.confirm("您是群主，退出将解散群组。确定继续吗？")) return;
      try {
        const api = await import("@/services/api");
        await api.dismissGroup(group.groupId);
        onBack();
        return;
      } catch { /* fallback */ }
      onBack();
      return;
    }
    if (!window.confirm("确定要退出该群聊吗？")) return;
    try {
      const api = await import("@/services/api");
      await api.quitGroup(group.groupId);
      onBack();
    } catch { /* fallback */ }
    onBack();
  };

  const handleInvite = async () => {
    // 打开邀请对话框 — 简化版，提示用户
    const name = window.prompt("输入要邀请的用户ID：");
    if (!name) return;
    try {
      const api = await import("@/services/api");
      await api.inviteToGroup(group.groupId, [name]);
      loadMembers();
    } catch { /* fallback */ }
  };

  const handleDismissGroup = async () => {
    if (!window.confirm("确定要解散该群聊吗？此操作不可撤销！")) return;
    try {
      const api = await import("@/services/api");
      await api.dismissGroup(group.groupId);
      onBack();
    } catch { /* fallback */ }
    onBack();
  };

  const handleApplicationAction = async (app: GroupApplication, accept: boolean) => {
    try {
      const api = await import("@/services/api");
      await api.handleApplication(app.applicationId, accept);
      setApplications((prev) => prev.filter((a) => a.applicationId !== app.applicationId));
      if (accept) loadMembers();
    } catch { /* fallback */ }
  };

  const roleLabel = (level: number) => {
    if (level === 2) return "群主";
    if (level === 1) return "管理员";
    return "成员";
  };

  const roleColor = (level: number) => {
    if (level === 2) return "text-amber-600 bg-amber-50";
    if (level === 1) return "text-indigo-600 bg-indigo-50";
    return "text-gray-500 bg-gray-100";
  };

  return (
    <div className="h-full flex flex-col bg-white w-full">
      {/* 头部 */}
      <div className="h-14 flex items-center gap-2 px-4 border-b border-gray-100 flex-shrink-0">
        <button
          onClick={onBack}
          className="p-1.5 -ml-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
        <div className="flex-1 min-w-0">
          <h2 className="text-sm font-semibold text-gray-900 truncate">{group.name}</h2>
          <p className="text-[10px] text-gray-400">{members.length} 名成员</p>
        </div>
        <button
          onClick={() => onOpenChat(group)}
          className="px-3 py-1.5 text-xs font-medium text-white bg-indigo-500 hover:bg-indigo-600 rounded-lg transition-colors"
        >
          发消息
        </button>
      </div>

      {/* 标签切换 */}
      <div className="flex border-b border-gray-100 px-4">
        {[
          { id: "members" as DetailTab, label: "成员" },
          { id: "applications" as DetailTab, label: "入群申请", badge: appCount },
          ...(isOwnerOrAdmin ? [{ id: "settings" as DetailTab, label: "设置" }] : []),
        ].map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              "relative px-4 py-2.5 text-sm font-medium transition-colors",
              tab === t.id ? "text-indigo-600" : "text-gray-400 hover:text-gray-600"
            )}
          >
            {t.label}
            {t.badge !== undefined && t.badge > 0 && (
              <span className="ml-1.5 px-1.5 py-0.5 text-xs font-bold text-white bg-red-500 rounded-full">
                {t.badge}
              </span>
            )}
            {tab === t.id && <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-indigo-500 rounded-full" />}
          </button>
        ))}
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-y-auto">
        {/* ---- 成员列表 ---- */}
        {tab === "members" && (
          loading ? (
            <div className="flex items-center justify-center py-20">
              <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
            </div>
          ) : (
            <div className="pb-20">
              {/* 操作按钮 */}
              {isOwnerOrAdmin && (
                <div className="px-4 pt-3 pb-2 flex gap-2">
                  <button
                    onClick={handleInvite}
                    className="flex-1 py-2 text-xs font-medium text-indigo-600 bg-indigo-50 hover:bg-indigo-100 rounded-lg transition-colors"
                  >
                    <UserPlus className="w-3.5 h-3.5 inline mr-1" />
                    邀请成员
                  </button>
                  <button
                    onClick={handleQuitGroup}
                    className="flex-1 py-2 text-xs font-medium text-red-500 bg-red-50 hover:bg-red-100 rounded-lg transition-colors"
                  >
                    <LogOut className="w-3.5 h-3.5 inline mr-1" />
                    {isOwner ? "解散群" : "退出群"}
                  </button>
                </div>
              )}
              {/* 成员项 */}
              {members.map((m) => (
                <div key={m.userId} className="flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 transition-colors group">
                  <UserAvatar name={m.displayName || m.userId} size="md" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-gray-900 truncate">
                        {m.displayName || m.userId}
                      </span>
                      {m.username && m.displayName !== m.username && (
                        <span className="text-xs text-gray-400">@{m.username}</span>
                      )}
                    </div>
                    <span className={cn("text-[10px] px-1.5 py-0.5 rounded-full", roleColor(m.roleLevel))}>
                      {roleLabel(m.roleLevel)}
                      {m.muteEndTime > Date.now() && " · 已禁言"}
                    </span>
                  </div>
                  {/* 操作菜单（群主/管理员可见） */}
                  {isOwnerOrAdmin && m.userId !== currentUser?.userId && (
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      {isOwner && m.roleLevel < 2 && (
                        <button
                          onClick={() => handleTransferOwner(m.userId)}
                          className="p-1.5 rounded-lg hover:bg-amber-50 text-gray-400 hover:text-amber-600"
                          title="转让群主"
                        >
                          <Crown className="w-3.5 h-3.5" />
                        </button>
                      )}
                      {m.roleLevel < (currentMember?.roleLevel ?? 0) && (
                        <button
                          onClick={() => handleKickMember(m.userId)}
                          className="p-1.5 rounded-lg hover:bg-red-50 text-gray-400 hover:text-red-500"
                          title="踢出"
                        >
                          <UserX className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  )}
                </div>
              ))}
              {/* 非管理成员显示退出按钮 */}
              {!isOwnerOrAdmin && (
                <div className="px-4 pt-4">
                  <button
                    onClick={handleQuitGroup}
                    className="w-full py-2 text-xs font-medium text-red-500 bg-red-50 hover:bg-red-100 rounded-lg transition-colors"
                  >
                    <LogOut className="w-3.5 h-3.5 inline mr-1" />
                    退出群聊
                  </button>
                </div>
              )}
            </div>
          )
        )}

        {/* ---- 入群申请 ---- */}
        {tab === "applications" && (
          applications.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
              <p>暂无待处理申请</p>
            </div>
          ) : (
            <div className="px-3 py-3 space-y-2">
              {applications.map((app) => (
                <div key={app.applicationId} className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80">
                  <UserAvatar name={app.user?.displayName || app.userId} size="md" />
                  <div className="flex-1 min-w-0">
                    <h4 className="text-sm font-medium text-gray-900">{app.user?.displayName || app.userId}</h4>
                    {app.message && <p className="text-xs text-gray-500 mt-1">{app.message}</p>}
                    <p className="text-[10px] text-gray-400 mt-1">{formatTimeAgo(app.createdAt)}</p>
                    <div className="flex gap-2 mt-2.5">
                      <button
                        onClick={() => handleApplicationAction(app, true)}
                        className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-white bg-green-500 hover:bg-green-600 rounded-lg transition-colors"
                      >
                        <Check className="w-3 h-3" /> 同意
                      </button>
                      <button
                        onClick={() => handleApplicationAction(app, false)}
                        className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-gray-600 bg-white border border-gray-200 hover:bg-gray-50 rounded-lg transition-colors"
                      >
                        <X className="w-3 h-3" /> 拒绝
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )
        )}

        {/* ---- 设置 ---- */}
        {tab === "settings" && (
          <div className="p-4 space-y-4">
            {/* 群信息 */}
            <div>
              <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">群信息</h4>
              <div className="space-y-3 bg-gray-50 rounded-xl p-3">
                <InfoRow label="群名称" value={group.name} />
                <InfoRow label="群ID" value={group.groupId} />
                <InfoRow label="成员数" value={`${members.length} 人`} />
                <InfoRow label="创建时间" value={new Date(group.createdAt).toLocaleDateString("zh-CN")} />
                {group.introduction && <InfoRow label="群简介" value={group.introduction} />}
                {group.notification && <InfoRow label="群公告" value={group.notification} />}
              </div>
            </div>

            {/* 操作 */}
            <div>
              <h4 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-2">管理操作</h4>
              <div className="space-y-2">
                <button
                  onClick={handleInvite}
                  className="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-gray-700 bg-gray-50 hover:bg-gray-100 rounded-xl transition-colors text-left"
                >
                  <UserPlus className="w-4 h-4 text-indigo-400" />
                  邀请成员
                </button>
                {isOwner && (
                  <button
                    onClick={handleDismissGroup}
                    className="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-red-600 bg-red-50 hover:bg-red-100 rounded-xl transition-colors text-left"
                  >
                    <Trash2 className="w-4 h-4" />
                    解散群聊
                  </button>
                )}
                {!isOwner && (
                  <button
                    onClick={handleQuitGroup}
                    className="w-full flex items-center gap-3 px-3 py-2.5 text-sm text-red-600 bg-red-50 hover:bg-red-100 rounded-xl transition-colors text-left"
                  >
                    <LogOut className="w-4 h-4" />
                    退出群聊
                  </button>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ---- 信息行 ----
function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-xs text-gray-400 min-w-[56px]">{label}</span>
      <span className="text-xs text-gray-700 break-all">{value || "-"}</span>
    </div>
  );
}

// ============================================================
// 入群申请面板（跨所有群组）
// ============================================================
function ApplicationsPanel({
  groups,
  onHandled,
}: {
  groups: Group[];
  onHandled: () => void;
}) {
  const [allApps, setAllApps] = useState<(GroupApplication & { groupName: string })[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        const api = await import("@/services/api");
        // 拉取所有群的待处理申请
        const results = await Promise.allSettled(
          groups.map(async (g) => {
            const apps = await api.getPendingApplications(g.groupId);
            return apps
              .filter((a) => a.status === "pending")
              .map((a) => ({ ...a, groupName: g.name }));
          })
        );
        const merged: (GroupApplication & { groupName: string })[] = [];
        results.forEach((r) => {
          if (r.status === "fulfilled") merged.push(...r.value);
        });
        setAllApps(merged);
      } catch { /* ignore */ }
      finally { setLoading(false); }
    };
    load();
  }, [groups]);

  const handleAction = async (app: GroupApplication & { groupName: string }, accept: boolean) => {
    try {
      const api = await import("@/services/api");
      await api.handleApplication(app.applicationId, accept);
      setAllApps((prev) => prev.filter((a) => a.applicationId !== app.applicationId));
      onHandled();
    } catch { /* fallback */ }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="w-5 h-5 text-indigo-500 animate-spin" />
        <span className="ml-2 text-sm text-gray-400">加载中...</span>
      </div>
    );
  }

  if (allApps.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 text-gray-400 text-sm">
        <Users className="w-12 h-12 mb-3 opacity-30" />
        <p>暂无待处理的入群申请</p>
      </div>
    );
  }

  return (
    <div className="px-3 py-3 space-y-2">
      {allApps.map((app) => (
        <div key={app.applicationId} className="flex items-start gap-3 p-3 rounded-xl bg-gray-50/80">
          <UserAvatar name={app.user?.displayName || app.userId} size="md" />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <h4 className="text-sm font-medium text-gray-900">{app.user?.displayName || app.userId}</h4>
              <span className="text-[10px] px-1.5 py-0.5 bg-indigo-100 text-indigo-600 rounded-full">
                {app.groupName}
              </span>
            </div>
            {app.message && <p className="text-xs text-gray-500 mt-1">{app.message}</p>}
            <p className="text-[10px] text-gray-400 mt-1">{formatTimeAgo(app.createdAt)}</p>
            <div className="flex gap-2 mt-2.5">
              <button
                onClick={() => handleAction(app, true)}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-white bg-green-500 hover:bg-green-600 rounded-lg transition-colors"
              >
                <Check className="w-3 h-3" /> 同意
              </button>
              <button
                onClick={() => handleAction(app, false)}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-gray-600 bg-white border border-gray-200 hover:bg-gray-50 rounded-lg transition-colors"
              >
                <X className="w-3 h-3" /> 拒绝
              </button>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// ============================================================
// 创建群聊对话框（嵌入版）
// ============================================================
function CreateGroupDialogEmbedded({
  contacts,
  currentUser,
  onCreateGroup,
  onClose,
}: {
  contacts: any[];
  currentUser: any;
  onCreateGroup: (name: string, memberIds: string[]) => Promise<void>;
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);

  const filtered = contacts.filter(
    (c) =>
      c.displayName.toLowerCase().includes(search.toLowerCase()) ||
      c.username.toLowerCase().includes(search.toLowerCase())
  );

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleCreate = async () => {
    if (!name.trim() || selected.size === 0) return;
    setCreating(true);
    await onCreateGroup(name.trim(), Array.from(selected));
    setCreating(false);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-100">
          <h3 className="font-semibold text-gray-900">创建群聊</h3>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-gray-100 text-gray-400">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 flex-1 overflow-y-auto">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">群聊名称</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入群聊名称"
              className="w-full px-4 py-2.5 rounded-xl border border-gray-200 text-sm
                focus:outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            />
          </div>

          {selected.size > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                已选 {selected.size} 人
              </label>
              <div className="flex flex-wrap gap-1.5">
                {Array.from(selected).map((id) => {
                  const c = contacts.find((x) => x.userId === id);
                  return c ? (
                    <span key={id} className="flex items-center gap-1 bg-indigo-50 text-indigo-600 px-2 py-1 rounded-lg text-xs">
                      {c.displayName}
                      <button onClick={() => toggle(id)} className="hover:text-red-500"><X className="w-3 h-3" /></button>
                    </span>
                  ) : null;
                })}
              </div>
            </div>
          )}

          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索联系人..."
              className="w-full pl-9 pr-4 py-2 rounded-xl border border-gray-200 text-sm
                focus:outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            />
          </div>

          <div className="space-y-0.5 max-h-48 overflow-y-auto -mx-2">
            {filtered.map((contact) => (
              <button
                key={contact.userId}
                onClick={() => toggle(contact.userId)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2 rounded-xl text-left transition-colors",
                  selected.has(contact.userId) ? "bg-indigo-50" : "hover:bg-gray-50"
                )}
              >
                <UserAvatar name={contact.displayName} size="md" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-900 truncate">{contact.displayName}</p>
                  <p className="text-xs text-gray-400">@{contact.username}</p>
                </div>
                <div className={cn(
                  "w-5 h-5 rounded-full border-2 flex items-center justify-center transition-colors",
                  selected.has(contact.userId) ? "bg-indigo-500 border-indigo-500" : "border-gray-300"
                )}>
                  {selected.has(contact.userId) && <Check className="w-3 h-3 text-white" />}
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="px-5 py-4 border-t border-gray-100 flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 py-2.5 rounded-xl border border-gray-200 text-sm font-medium text-gray-600 hover:bg-gray-50 transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleCreate}
            disabled={!name.trim() || selected.size === 0 || creating}
            className={cn(
              "flex-1 py-2.5 rounded-xl text-sm font-medium text-white transition-all",
              name.trim() && selected.size > 0 && !creating
                ? "bg-indigo-500 hover:bg-indigo-600 active:scale-[0.98] shadow-md shadow-indigo-200"
                : "bg-gray-300 cursor-not-allowed"
            )}
          >
            {creating ? (
              <><Loader2 className="w-4 h-4 animate-spin inline mr-1" />创建中</>
            ) : "创建群聊"}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------- 时间格式化 ----------
function formatTimeAgo(dateStr: string): string {
  if (!dateStr) return "";
  const d = new Date(dateStr).getTime();
  if (isNaN(d)) return "";
  const diff = Date.now() - d;
  if (diff < 60000) return "刚刚";
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`;
  return new Date(dateStr).toLocaleDateString("zh-CN");
}
