"use client";

import React, { useEffect, useRef, useState } from "react";
import {
  ArrowLeft,
  Ban,
  BellOff,
  Camera,
  Crown,
  Info,
  Loader2,
  LogOut,
  MoreHorizontal,
  Phone,
  Pin,
  Search,
  ShieldCheck,
  Trash2,
  UserMinus,
  UsersRound,
  Video,
  X,
} from "lucide-react";
import type { Conversation, Group, GroupMemberInfo } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

type Props = {
  conversation: Conversation;
  onBack?: () => void;
  onToggleMute?: (next: boolean) => void;
  onTogglePin?: (next: boolean) => void;
};

export default function ChatHeader({ conversation, onBack, onToggleMute, onTogglePin }: Props) {
  const { user } = useAuth();
  const {
    contacts,
    groups,
    refreshGroups,
    updateConversation,
    removeConversation,
    refreshContacts,
    refreshConversations,
    startVoiceCall,
    callPhase,
  } = useChat();
  const [showInfo, setShowInfo] = useState(false);
  const [showMore, setShowMore] = useState(false);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");
  const [isBlocked, setIsBlocked] = useState(false);
  const [transferring, setTransferring] = useState(false);
  const moreRef = useRef<HTMLDivElement>(null);
  const blackSyncSeq = useRef(0);

  const [group, setGroup] = useState<Group>();
  const [members, setMembers] = useState<GroupMemberInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [name, setName] = useState("");
  const [introduction, setIntroduction] = useState("");
  const [notification, setNotification] = useState("");
  const [needVerification, setNeedVerification] = useState(false);
  const [avatarFile, setAvatarFile] = useState<File>();
  const [avatarPreview, setAvatarPreview] = useState("");
  const avatarInput = useRef<HTMLInputElement>(null);

  const isPrivate = conversation.type === "private";
  const otherId = conversation.members.find((item) => item.userId !== user?.userId)?.userId;
  const other = contacts.find((item) => item.userId === otherId);
  const isFriend = Boolean(otherId && other?.isFriend);
  const groupId =
    conversation.groupId ||
    (conversation.type === "group" ? IMSDK.parseGroupId(conversation.conversationId) : "");
  const knownGroup = groups.find((item) => item.groupId === groupId);
  const memberCount =
    group?.memberCount || knownGroup?.memberCount || members.length || conversation.members.length;
  const status =
    conversation.type === "group"
      ? `${memberCount} 名成员`
      : other?.status === "online"
        ? "在线"
        : other?.status === "away"
          ? "离开"
          : "离线";
  const currentMember = members.find((item) => item.userId === user?.userId);
  // 信息面板已加载时以 group/members 为准，避免转让后 knownGroup 短暂滞后
  const isOwner =
    conversation.type === "group" &&
    (group
      ? group.ownerId === user?.userId || currentMember?.roleLevel === 2
      : knownGroup?.ownerId === user?.userId);
  const canEdit =
    conversation.type === "group" &&
    (isOwner || (currentMember?.roleLevel || 0) >= 1 || knownGroup?.ownerId === user?.userId);
  const iconButton =
    "ui-press flex h-9 w-9 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted hover:text-ink";

  useEffect(() => {
    if (!showMore) return;
    const onDoc = (e: MouseEvent) => {
      if (moreRef.current && !moreRef.current.contains(e.target as Node)) setShowMore(false);
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [showMore]);

  useEffect(() => {
    setIsBlocked(false);
    setActionError("");
  }, [conversation.conversationId, otherId]);

  useEffect(() => {
    if (!showMore || !isPrivate || !otherId) return;
    const seq = ++blackSyncSeq.current;
    void (async () => {
      try {
        const list = await IMSDK.getBlackList({ offset: 0, limit: 200 });
        if (seq !== blackSyncSeq.current) return;
        setIsBlocked(list.some((item) => item.userId === otherId));
      } catch {
        // keep current switch state
      }
    })();
  }, [showMore, isPrivate, otherId]);

  useEffect(() => {
    if (!showInfo || conversation.type !== "group" || !groupId) return;
    let active = true;
    setLoading(true);
    setError("");
    Promise.all([
      IMSDK.getGroupInfo(groupId),
      IMSDK.getGroupMemberList(groupId),
    ])
      .then(([nextGroup, nextMembers]) => {
        if (!active) return;
        setGroup(nextGroup);
        setMembers(nextMembers);
        setName(nextGroup.name);
        setIntroduction(nextGroup.introduction || "");
        setNotification(nextGroup.notification || "");
        setNeedVerification(Boolean(nextGroup.needVerification));
      })
      .catch((err) => active && setError(err instanceof Error ? err.message : "群信息加载失败"))
      .finally(() => active && setLoading(false));

    const unsub = IMSDK.on("group.member.synced", (msg) => {
      const payload = msg.payload as
        | { groupId?: string; members?: GroupMemberInfo[] }
        | undefined;
      if (!active || payload?.groupId !== groupId || !payload.members) return;
      setMembers(payload.members);
    });
    return () => {
      active = false;
      unsub();
    };
  }, [showInfo, groupId, conversation.type]);

  useEffect(
    () => () => {
      if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    },
    [avatarPreview]
  );

  const chooseAvatar = (file?: File) => {
    if (!file) return;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    setAvatarFile(file);
    setAvatarPreview(URL.createObjectURL(file));
    setError("");
  };

  const saveGroup = async () => {
    if (!group || !canEdit || !name.trim()) return;
    setSaving(true);
    setError("");
    try {
      let avatar = group.avatar;
      if (avatarFile) avatar = await IMSDK.uploadAvatar(avatarFile, { type: "group", id: group.groupId });
      await IMSDK.setGroupInfo({
        groupId: group.groupId,
        name: name.trim(),
        introduction: introduction.trim(),
        notification: notification.trim(),
        needVerification,
      });
      const updated = await IMSDK.getGroupInfo(group.groupId);
      setGroup(updated);
      setAvatarFile(undefined);
      setAvatarPreview("");
      updateConversation(conversation.conversationId, {
        title: updated.name,
        avatar: avatar || updated.avatar,
      });
      await refreshGroups();
    } catch (err) {
      setError(err instanceof Error ? err.message : "群信息保存失败");
    } finally {
      setSaving(false);
    }
  };

  const closeChat = () => {
    removeConversation(conversation.conversationId);
    onBack?.();
  };

  const handleToggleBlack = async (enabled: boolean) => {
    if (!otherId || busy || enabled === isBlocked) return;
    blackSyncSeq.current += 1; // 忽略进行中的黑名单同步，避免覆盖乐观状态
    setBusy(true);
    setActionError("");
    const prev = isBlocked;
    setIsBlocked(enabled);
    try {
      if (enabled) {
        // 拉黑保留好友关系；仅写入黑名单，开关保持开启
        await IMSDK.addBlack(otherId);
      } else {
        await IMSDK.removeBlack(otherId);
      }
    } catch (e) {
      setIsBlocked(prev);
      setActionError(
        e instanceof Error ? e.message : enabled ? "加入黑名单失败" : "移出黑名单失败"
      );
    } finally {
      setBusy(false);
    }
  };

  const handleDeleteFriend = async () => {
    if (!otherId || busy) return;
    if (!window.confirm(`删除好友 ${conversation.title}？`)) return;
    setBusy(true);
    setActionError("");
    setShowMore(false);
    try {
      await IMSDK.deleteFriend(otherId);
      await refreshContacts();
      closeChat();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "删除好友失败");
    } finally {
      setBusy(false);
    }
  };

  const handleLeaveGroup = async () => {
    if (!groupId || busy || isOwner) return;
    if (!window.confirm(`退出群聊「${conversation.title}」？退出后仍可查看历史消息。`)) return;
    setBusy(true);
    setActionError("");
    setShowMore(false);
    setShowInfo(false);
    try {
      await IMSDK.quitGroup(groupId);
      await refreshGroups();
      await refreshConversations();
      onBack?.();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "退出群聊失败");
    } finally {
      setBusy(false);
    }
  };

  const handleDismissGroup = async () => {
    if (!groupId || busy || !isOwner) return;
    if (!window.confirm(`解散群聊「${conversation.title}」？此操作不可撤销。`)) return;
    setBusy(true);
    setActionError("");
    setShowMore(false);
    setShowInfo(false);
    try {
      await IMSDK.dismissGroup(groupId);
      await refreshGroups();
      closeChat();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "解散群聊失败");
    } finally {
      setBusy(false);
    }
  };

  const handleTransferOwner = async (member: GroupMemberInfo) => {
    if (!groupId || busy || !isOwner || member.userId === user?.userId) return;
    const label = member.displayName || member.userId;
    if (!window.confirm(`将群主转让给「${label}」？转让后你将成为管理员，可再退出群聊。`)) return;
    setBusy(true);
    setTransferring(true);
    setActionError("");
    setError("");
    try {
      await IMSDK.transferGroupOwner(groupId, member.userId);
      const [nextGroup, nextMembers] = await Promise.all([
        IMSDK.getGroupInfo(groupId),
        IMSDK.getGroupMemberList(groupId),
      ]);
      setGroup(nextGroup);
      setMembers(nextMembers);
      await refreshGroups();
    } catch (e) {
      setError(e instanceof Error ? e.message : "转让群主失败");
    } finally {
      setTransferring(false);
      setBusy(false);
    }
  };

  const menuRowClass =
    "flex w-full items-center gap-2.5 px-3 py-2.5 text-sm text-ink";
  const menuItemClass =
    "flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm text-ink hover:bg-surface-muted disabled:opacity-50";

  return (
    <>
      <header className="flex h-[68px] flex-shrink-0 items-center justify-between border-b border-edge bg-surface-elevated px-3 sm:px-5">
        <div className="flex min-w-0 items-center gap-2.5">
          <button onClick={onBack} className={`${iconButton} md:hidden`} title="返回">
            <ArrowLeft className="h-5 w-5" strokeWidth={1.75} />
          </button>
          {conversation.type === "group" && !conversation.avatar ? (
            <div className="flex h-10 w-10 items-center justify-center rounded-control bg-accent-soft text-accent">
              <UsersRound className="h-5 w-5" strokeWidth={1.75} />
            </div>
          ) : (
            <UserAvatar src={conversation.avatar} name={conversation.title} size="md" />
          )}
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="truncate text-sm font-semibold text-ink">{conversation.title}</h2>
              {conversation.isPinned && <Pin className="h-3.5 w-3.5 text-amber-500" strokeWidth={1.75} />}
            </div>
            <p className="mt-0.5 flex items-center gap-1.5 text-xs text-ink-muted">
              <span
                className={cn(
                  "h-1.5 w-1.5 rounded-full",
                  status === "在线" ? "bg-accent" : "bg-ink-muted/40"
                )}
              />
              {status}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-0.5">
          <button className={`${iconButton} hidden sm:flex`} title="搜索聊天记录">
            <Search className="h-[18px] w-[18px]" strokeWidth={1.75} />
          </button>
          <button
            onClick={() => {
              if (!isPrivate || !otherId || callPhase !== "idle") return;
              void startVoiceCall(otherId, conversation.title, conversation.avatar);
            }}
            disabled={!isPrivate || !otherId || !isFriend || callPhase !== "idle" || busy}
            className={cn(iconButton, "disabled:opacity-40")}
            title="语音通话"
          >
            <Phone className="h-[18px] w-[18px]" strokeWidth={1.75} />
          </button>
          <button
            className={cn(iconButton, "hidden sm:flex disabled:opacity-40")}
            disabled
            title="视频通话（即将推出）"
          >
            <Video className="h-[18px] w-[18px]" strokeWidth={1.75} />
          </button>
          <button
            onClick={() => {
              setShowMore(false);
              setShowInfo(true);
            }}
            className={cn(iconButton, showInfo && "bg-surface-muted text-ink")}
            title="会话详情"
          >
            <Info className="h-[18px] w-[18px]" strokeWidth={1.75} />
          </button>

          <div className="relative" ref={moreRef}>
            <button
              onClick={() => setShowMore((v) => !v)}
              className={cn(iconButton, showMore && "bg-surface-muted text-ink")}
              title="更多操作"
              disabled={busy}
            >
              <MoreHorizontal className="h-[18px] w-[18px]" strokeWidth={1.75} />
            </button>
            {showMore ? (
              <div className="absolute right-0 top-full z-30 mt-1 w-56 overflow-hidden rounded-control border border-edge bg-surface-elevated py-1 shadow-panel">
                <div className={menuRowClass}>
                  <Pin className="h-4 w-4 flex-none text-ink-muted" strokeWidth={1.75} />
                  <span className="min-w-0 flex-1">设为置顶</span>
                  <SlideSwitch
                    checked={Boolean(conversation.isPinned)}
                    disabled={busy}
                    onCheckedChange={(next) => onTogglePin?.(next)}
                    ariaLabel="设为置顶"
                  />
                </div>
                <div className={menuRowClass}>
                  <BellOff className="h-4 w-4 flex-none text-ink-muted" strokeWidth={1.75} />
                  <span className="min-w-0 flex-1">消息免打扰</span>
                  <SlideSwitch
                    checked={Boolean(conversation.isMuted)}
                    disabled={busy}
                    onCheckedChange={(next) => onToggleMute?.(next)}
                    ariaLabel="消息免打扰"
                  />
                </div>
                {isPrivate ? (
                  <>
                    <div className="my-1 border-t border-edge" />
                    <div className={menuRowClass}>
                      <Ban className="h-4 w-4 flex-none text-ink-muted" strokeWidth={1.75} />
                      <span className="min-w-0 flex-1">加入黑名单</span>
                      <SlideSwitch
                        checked={isBlocked}
                        disabled={!otherId || busy}
                        onCheckedChange={(next) => void handleToggleBlack(next)}
                        ariaLabel="加入黑名单"
                      />
                    </div>
                    <button
                      type="button"
                      className={cn(menuItemClass, "text-danger hover:bg-danger-soft")}
                      disabled={!otherId || !isFriend || busy}
                      onClick={() => void handleDeleteFriend()}
                    >
                      <UserMinus className="h-4 w-4" strokeWidth={1.75} />
                      删除好友
                    </button>
                  </>
                ) : null}
                {conversation.type === "group" ? (
                  <>
                    <div className="my-1 border-t border-edge" />
                    {isOwner ? (
                      <button
                        type="button"
                        className={cn(menuItemClass, "text-danger hover:bg-danger-soft")}
                        disabled={!groupId || busy}
                        onClick={() => void handleDismissGroup()}
                      >
                        <Trash2 className="h-4 w-4" strokeWidth={1.75} />
                        解散群聊
                      </button>
                    ) : (
                      <button
                        type="button"
                        className={cn(menuItemClass, "text-danger hover:bg-danger-soft")}
                        disabled={!groupId || busy}
                        onClick={() => void handleLeaveGroup()}
                      >
                        <LogOut className="h-4 w-4" strokeWidth={1.75} />
                        退出群聊
                      </button>
                    )}
                  </>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
      </header>

      {actionError ? (
        <div className="border-b border-danger-soft bg-danger-soft px-4 py-2 text-xs text-danger">{actionError}</div>
      ) : null}

      {showInfo && (
        <aside className="absolute bottom-0 right-0 top-[68px] z-20 flex w-full flex-col border-l border-edge bg-surface-elevated shadow-panel sm:w-[360px]">
          <div className="flex h-14 flex-none items-center justify-between border-b border-edge px-5">
            <div>
              <h3 className="text-sm font-semibold text-ink">
                {conversation.type === "group" ? "群聊信息" : "会话详情"}
              </h3>
              {conversation.type === "group" && canEdit && (
                <p className="text-[11px] text-accent">可编辑</p>
              )}
            </div>
            <button onClick={() => setShowInfo(false)} className={iconButton} title="关闭">
              <X className="h-4 w-4" strokeWidth={1.75} />
            </button>
          </div>
          <div className="flex-1 overflow-y-auto px-5 py-5">
            {loading ? (
              <div className="flex h-40 items-center justify-center">
                <Loader2 className="h-6 w-6 animate-spin text-accent" strokeWidth={1.75} />
              </div>
            ) : conversation.type === "group" ? (
              <>
                <div className="flex flex-col items-center border-b border-edge pb-5">
                  <button
                    disabled={!canEdit}
                    onClick={() => avatarInput.current?.click()}
                    className="group/avatar relative rounded-control"
                    title={canEdit ? "更换群头像" : "群头像"}
                  >
                    {avatarPreview || group?.avatar || conversation.avatar ? (
                      <UserAvatar
                        src={avatarPreview || group?.avatar || conversation.avatar}
                        name={name || conversation.title}
                        size="xl"
                        className="h-20 w-20"
                      />
                    ) : (
                      <div className="flex h-20 w-20 items-center justify-center rounded-control bg-accent-soft text-accent">
                        <UsersRound className="h-8 w-8" strokeWidth={1.75} />
                      </div>
                    )}
                    {canEdit && (
                      <span className="absolute inset-0 flex items-center justify-center rounded-control bg-ink/0 text-transparent transition group-hover/avatar:bg-ink/40 group-hover/avatar:text-surface-elevated">
                        <Camera className="h-5 w-5" strokeWidth={1.75} />
                      </span>
                    )}
                  </button>
                  <input
                    ref={avatarInput}
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="hidden"
                    onChange={(event) => chooseAvatar(event.target.files?.[0])}
                  />
                  <p className="mt-3 text-xs text-ink-muted">{memberCount} 名成员</p>
                </div>
                <div className="space-y-4 border-b border-edge py-5">
                  <label className="block">
                    <span className="mb-1.5 block text-xs font-medium text-ink-muted">群名称</span>
                    <input
                      disabled={!canEdit}
                      value={name}
                      maxLength={64}
                      onChange={(e) => setName(e.target.value)}
                      className="h-10 w-full rounded-control border border-edge px-3 text-sm outline-none focus:border-accent disabled:bg-surface-muted disabled:text-ink-muted"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-1.5 block text-xs font-medium text-ink-muted">群简介</span>
                    <textarea
                      disabled={!canEdit}
                      value={introduction}
                      maxLength={300}
                      onChange={(e) => setIntroduction(e.target.value)}
                      rows={3}
                      className="w-full resize-none rounded-control border border-edge px-3 py-2 text-sm outline-none focus:border-accent disabled:bg-surface-muted disabled:text-ink-muted"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-1.5 block text-xs font-medium text-ink-muted">群公告</span>
                    <textarea
                      disabled={!canEdit}
                      value={notification}
                      maxLength={500}
                      onChange={(e) => setNotification(e.target.value)}
                      rows={3}
                      className="w-full resize-none rounded-control border border-edge px-3 py-2 text-sm outline-none focus:border-accent disabled:bg-surface-muted disabled:text-ink-muted"
                    />
                  </label>
                  {canEdit && (
                    <label className="flex h-10 items-center justify-between text-sm text-ink">
                      <span>入群需要验证</span>
                      <input
                        type="checkbox"
                        checked={needVerification}
                        onChange={(e) => setNeedVerification(e.target.checked)}
                        className="h-4 w-4 accent-accent"
                      />
                    </label>
                  )}
                  {error && <p className="text-xs text-danger">{error}</p>}
                  {canEdit && (
                    <button
                      disabled={saving}
                      onClick={saveGroup}
                      className="ui-press flex h-10 w-full items-center justify-center gap-2 rounded-control bg-accent text-sm font-medium text-accent-fg hover:bg-accent-hover disabled:opacity-60"
                    >
                      {saving && <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} />}
                      保存群信息
                    </button>
                  )}
                </div>
                <div className="py-5">
                  <div className="mb-3 flex items-center justify-between">
                    <p className="text-xs font-semibold text-ink-muted">群成员</p>
                    <span className="text-xs text-ink-muted">{members.length}</span>
                  </div>
                  {isOwner ? (
                    <p className="mb-2 text-[11px] text-ink-muted">点击成员可转让群主</p>
                  ) : null}
                  <div className="space-y-1">
                    {members.map((item) => {
                      const canTransfer = isOwner && item.userId !== user?.userId;
                      return (
                        <button
                          key={item.userId}
                          type="button"
                          disabled={!canTransfer || busy || transferring}
                          onClick={() => void handleTransferOwner(item)}
                          className={cn(
                            "flex h-11 w-full items-center gap-3 rounded-control px-1 text-left",
                            canTransfer && "hover:bg-surface-muted disabled:opacity-50"
                          )}
                        >
                          <UserAvatar src={item.avatar} name={item.displayName || item.userId} size="sm" />
                          <p className="min-w-0 flex-1 truncate text-sm text-ink">
                            {item.userId === user?.userId
                              ? `${item.displayName || "我"}（我）`
                              : item.displayName || item.userId}
                          </p>
                          {item.roleLevel === 2 ? (
                            <span className="flex items-center gap-1 text-[11px] text-amber-600">
                              <Crown className="h-3.5 w-3.5" strokeWidth={1.75} />
                              群主
                            </span>
                          ) : item.roleLevel === 1 ? (
                            <span className="flex items-center gap-1 text-[11px] text-accent">
                              <ShieldCheck className="h-3.5 w-3.5" strokeWidth={1.75} />
                              管理员
                            </span>
                          ) : null}
                        </button>
                      );
                    })}
                  </div>
                </div>
                <div className="border-t border-edge pt-4">
                  {isOwner ? (
                    <button
                      type="button"
                      disabled={!groupId || busy}
                      onClick={() => void handleDismissGroup()}
                      className="ui-press flex h-10 w-full items-center justify-center gap-2 rounded-control text-sm text-danger hover:bg-danger-soft disabled:opacity-50"
                    >
                      <Trash2 className="h-4 w-4" strokeWidth={1.75} />
                      解散群聊
                    </button>
                  ) : (
                    <button
                      type="button"
                      disabled={!groupId || busy}
                      onClick={() => void handleLeaveGroup()}
                      className="ui-press flex h-10 w-full items-center justify-center gap-2 rounded-control text-sm text-danger hover:bg-danger-soft disabled:opacity-50"
                    >
                      <LogOut className="h-4 w-4" strokeWidth={1.75} />
                      退出群聊
                    </button>
                  )}
                </div>
              </>
            ) : (
              <div className="flex flex-col items-center pb-5">
                <UserAvatar src={conversation.avatar} name={conversation.title} size="xl" />
                <p className="mt-3 text-sm font-semibold text-ink">{conversation.title}</p>
                <p className="mt-1 text-xs text-ink-muted">{status}</p>
              </div>
            )}
          </div>
        </aside>
      )}
    </>
  );
}

function SlideSwitch({
  checked,
  disabled,
  danger,
  onCheckedChange,
  ariaLabel,
}: {
  checked: boolean;
  disabled?: boolean;
  danger?: boolean;
  onCheckedChange: (next: boolean) => void;
  ariaLabel: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation();
        if (disabled) return;
        onCheckedChange(!checked);
      }}
      className={cn(
        "relative h-6 w-11 flex-none rounded-full transition-colors duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60 disabled:cursor-not-allowed disabled:opacity-50",
        checked ? (danger ? "bg-danger" : "bg-accent") : "bg-surface-muted"
      )}
    >
      <span
        className={cn(
          "pointer-events-none absolute top-0.5 left-0.5 h-5 w-5 rounded-full bg-surface-elevated shadow-sm transition-transform duration-200 ease-out",
          checked ? "translate-x-5" : "translate-x-0"
        )}
      />
    </button>
  );
}
