"use client";

import React, { useEffect, useRef, useState } from "react";
import { ArrowLeft, Bell, BellOff, Camera, Crown, Info, Loader2, MoreHorizontal, Phone, Pin, Search, ShieldCheck, UsersRound, Video, X } from "lucide-react";
import type { Conversation, Group, GroupMemberInfo } from "@/types";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import * as api from "@/services/api";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";

type Props = { conversation: Conversation; onBack?: () => void; onToggleMute?: () => void; onTogglePin?: () => void };

export default function ChatHeader({ conversation, onBack, onToggleMute, onTogglePin }: Props) {
  const { user } = useAuth();
  const { contacts, groups, refreshGroups, updateConversation } = useChat();
  const [showInfo, setShowInfo] = useState(false);
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

  const otherId = conversation.members.find((item) => item.userId !== user?.userId)?.userId;
  const other = contacts.find((item) => item.userId === otherId);
  const knownGroup = groups.find((item) => item.groupId === conversation.conversationId);
  const memberCount = group?.memberCount || knownGroup?.memberCount || members.length || conversation.members.length;
  const status = conversation.type === "group" ? `${memberCount} 名成员` : other?.status === "online" ? "在线" : other?.status === "away" ? "离开" : "离线";
  const currentMember = members.find((item) => item.userId === user?.userId);
  const canEdit = conversation.type === "group" && (group?.ownerId === user?.userId || (currentMember?.roleLevel || 0) >= 1);
  const iconButton = "flex h-9 w-9 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100 hover:text-slate-800";

  useEffect(() => {
    if (!showInfo || conversation.type !== "group") return;
    let active = true;
    setLoading(true); setError("");
    Promise.all([api.getGroupInfo(conversation.conversationId), api.getGroupMembers(conversation.conversationId)])
      .then(([nextGroup, nextMembers]) => {
        if (!active) return;
        setGroup(nextGroup); setMembers(nextMembers); setName(nextGroup.name); setIntroduction(nextGroup.introduction || ""); setNotification(nextGroup.notification || ""); setNeedVerification(Boolean(nextGroup.needVerification));
      })
      .catch((err) => active && setError(err instanceof Error ? err.message : "群信息加载失败"))
      .finally(() => active && setLoading(false));
    return () => { active = false; };
  }, [showInfo, conversation.conversationId, conversation.type]);
  useEffect(() => () => { if (avatarPreview) URL.revokeObjectURL(avatarPreview); }, [avatarPreview]);

  const chooseAvatar = (file?: File) => {
    if (!file) return;
    if (avatarPreview) URL.revokeObjectURL(avatarPreview);
    setAvatarFile(file); setAvatarPreview(URL.createObjectURL(file)); setError("");
  };
  const saveGroup = async () => {
    if (!group || !canEdit || !name.trim()) return;
    setSaving(true); setError("");
    try {
      let avatar = group.avatar;
      if (avatarFile) avatar = await api.uploadAvatar(avatarFile, { type: "group", id: group.groupId });
      await api.updateGroupInfo({ groupId: group.groupId, name: name.trim(), introduction: introduction.trim(), notification: notification.trim(), needVerification });
      const updated = await api.getGroupInfo(group.groupId);
      setGroup(updated); setAvatarFile(undefined); setAvatarPreview("");
      updateConversation(conversation.conversationId, { title: updated.name, avatar: avatar || updated.avatar });
      await refreshGroups();
    } catch (err) { setError(err instanceof Error ? err.message : "群信息保存失败"); }
    finally { setSaving(false); }
  };

  return <>
    <header className="flex h-[68px] flex-shrink-0 items-center justify-between border-b border-slate-200 bg-white px-3 sm:px-5">
      <div className="flex min-w-0 items-center gap-2.5"><button onClick={onBack} className={`${iconButton} md:hidden`} title="返回"><ArrowLeft className="h-5 w-5" /></button>{conversation.type === "group" && !conversation.avatar ? <div className="flex h-10 w-10 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-5 w-5" /></div> : <UserAvatar src={conversation.avatar} name={conversation.title} size="md" />}<div className="min-w-0"><div className="flex items-center gap-2"><h2 className="truncate text-sm font-semibold text-slate-900">{conversation.title}</h2>{conversation.isPinned && <Pin className="h-3.5 w-3.5 text-amber-500" />}</div><p className="mt-0.5 flex items-center gap-1.5 text-xs text-slate-400"><span className={cn("h-1.5 w-1.5 rounded-full", status === "在线" ? "bg-emerald-500" : "bg-slate-300")} />{status}</p></div></div>
      <div className="flex items-center gap-0.5"><button className={`${iconButton} hidden sm:flex`} title="搜索聊天记录"><Search className="h-[18px] w-[18px]" /></button><button onClick={() => window.alert(`正在呼叫 ${conversation.title}`)} className={iconButton} title="语音通话"><Phone className="h-[18px] w-[18px]" /></button><button className={`${iconButton} hidden sm:flex`} title="视频通话"><Video className="h-[18px] w-[18px]" /></button><button onClick={() => setShowInfo(true)} className={cn(iconButton, showInfo && "bg-slate-100 text-slate-900")} title="会话详情"><Info className="h-[18px] w-[18px]" /></button><button className={iconButton} title="更多操作"><MoreHorizontal className="h-[18px] w-[18px]" /></button></div>
    </header>

    {showInfo && <aside className="absolute bottom-0 right-0 top-[68px] z-20 flex w-full flex-col border-l border-slate-200 bg-white shadow-xl sm:w-[360px]">
      <div className="flex h-14 flex-none items-center justify-between border-b border-slate-100 px-5"><div><h3 className="text-sm font-semibold text-slate-900">{conversation.type === "group" ? "群聊信息" : "会话详情"}</h3>{conversation.type === "group" && canEdit && <p className="text-[11px] text-emerald-600">可编辑</p>}</div><button onClick={() => setShowInfo(false)} className={iconButton} title="关闭"><X className="h-4 w-4" /></button></div>
      <div className="flex-1 overflow-y-auto px-5 py-5">
        {loading ? <div className="flex h-40 items-center justify-center"><Loader2 className="h-6 w-6 animate-spin text-emerald-600" /></div> : conversation.type === "group" ? <>
          <div className="flex flex-col items-center border-b border-slate-100 pb-5"><button disabled={!canEdit} onClick={() => avatarInput.current?.click()} className="group/avatar relative rounded-full" title={canEdit ? "更换群头像" : "群头像"}>{avatarPreview || group?.avatar || conversation.avatar ? <UserAvatar src={avatarPreview || group?.avatar || conversation.avatar} name={name || conversation.title} size="xl" className="h-20 w-20" /> : <div className="flex h-20 w-20 items-center justify-center rounded-full bg-sky-100 text-sky-700"><UsersRound className="h-8 w-8" /></div>}{canEdit && <span className="absolute inset-0 flex items-center justify-center rounded-full bg-slate-950/0 text-transparent transition group-hover/avatar:bg-slate-950/40 group-hover/avatar:text-white"><Camera className="h-5 w-5" /></span>}</button><input ref={avatarInput} type="file" accept="image/jpeg,image/png,image/webp" className="hidden" onChange={(event) => chooseAvatar(event.target.files?.[0])} /><p className="mt-3 text-xs text-slate-400">{memberCount} 名成员</p></div>
          <div className="space-y-4 border-b border-slate-100 py-5"><label className="block"><span className="mb-1.5 block text-xs font-medium text-slate-500">群名称</span><input disabled={!canEdit} value={name} maxLength={64} onChange={(e) => setName(e.target.value)} className="h-10 w-full rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-emerald-500 disabled:bg-slate-50 disabled:text-slate-500" /></label><label className="block"><span className="mb-1.5 block text-xs font-medium text-slate-500">群简介</span><textarea disabled={!canEdit} value={introduction} maxLength={300} onChange={(e) => setIntroduction(e.target.value)} rows={3} className="w-full resize-none rounded-md border border-slate-200 px-3 py-2 text-sm outline-none focus:border-emerald-500 disabled:bg-slate-50 disabled:text-slate-500" /></label><label className="block"><span className="mb-1.5 block text-xs font-medium text-slate-500">群公告</span><textarea disabled={!canEdit} value={notification} maxLength={500} onChange={(e) => setNotification(e.target.value)} rows={3} className="w-full resize-none rounded-md border border-slate-200 px-3 py-2 text-sm outline-none focus:border-emerald-500 disabled:bg-slate-50 disabled:text-slate-500" /></label>{canEdit && <label className="flex h-10 items-center justify-between text-sm text-slate-700"><span>入群需要验证</span><input type="checkbox" checked={needVerification} onChange={(e) => setNeedVerification(e.target.checked)} className="h-4 w-4 accent-emerald-600" /></label>}{error && <p className="text-xs text-rose-600">{error}</p>}{canEdit && <button disabled={saving} onClick={saveGroup} className="flex h-10 w-full items-center justify-center gap-2 rounded-md bg-emerald-600 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-60">{saving && <Loader2 className="h-4 w-4 animate-spin" />}保存群信息</button>}</div>
          <div className="border-b border-slate-100 py-5"><div className="mb-3 flex items-center justify-between"><p className="text-xs font-semibold text-slate-500">群成员</p><span className="text-xs text-slate-400">{members.length}</span></div><div className="space-y-1">{members.map((item) => <div key={item.userId} className="flex h-11 items-center gap-3"><UserAvatar src={item.avatar} name={item.displayName || item.userId} size="sm" /><p className="min-w-0 flex-1 truncate text-sm text-slate-700">{item.userId === user?.userId ? `${item.displayName || "我"}（我）` : item.displayName || item.userId}</p>{item.roleLevel === 2 ? <span className="flex items-center gap-1 text-[11px] text-amber-600"><Crown className="h-3.5 w-3.5" />群主</span> : item.roleLevel === 1 ? <span className="flex items-center gap-1 text-[11px] text-sky-600"><ShieldCheck className="h-3.5 w-3.5" />管理员</span> : null}</div>)}</div></div>
        </> : <div className="flex flex-col items-center border-b border-slate-100 pb-5"><UserAvatar src={conversation.avatar} name={conversation.title} size="xl" /><p className="mt-3 text-sm font-semibold text-slate-900">{conversation.title}</p><p className="mt-1 text-xs text-slate-400">{status}</p></div>}
        <div className="py-3"><button onClick={onTogglePin} className="flex h-11 w-full items-center justify-between text-sm text-slate-700"><span className="flex items-center gap-2"><Pin className="h-4 w-4 text-slate-400" />置顶会话</span><span className={cn("h-5 w-9 rounded-full p-0.5 transition", conversation.isPinned ? "bg-emerald-500" : "bg-slate-200")}><span className={cn("block h-4 w-4 rounded-full bg-white transition", conversation.isPinned && "translate-x-4")} /></span></button><button onClick={onToggleMute} className="flex h-11 w-full items-center justify-between text-sm text-slate-700"><span className="flex items-center gap-2">{conversation.isMuted ? <BellOff className="h-4 w-4 text-slate-400" /> : <Bell className="h-4 w-4 text-slate-400" />}消息免打扰</span><span className={cn("h-5 w-9 rounded-full p-0.5 transition", conversation.isMuted ? "bg-emerald-500" : "bg-slate-200")}><span className={cn("block h-4 w-4 rounded-full bg-white transition", conversation.isMuted && "translate-x-4")} /></span></button></div>
      </div>
    </aside>}
  </>;
}
