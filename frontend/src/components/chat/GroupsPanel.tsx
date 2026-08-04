"use client";

import React, { useCallback, useEffect, useState } from "react";
import { ArrowLeft, BellOff, Check, ChevronRight, Loader2, Plus, Search, ShieldCheck, UsersRound, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { IMSDK } from "@/suim-sdk";
import type { GroupApplication } from "@/types";
import { formatConvTime } from "@/lib/utils";
import CreateGroupDialog from "./CreateGroupDialog";
import UserAvatar from "../shared/UserAvatar";

interface GroupsPanelProps { onOpenConversation?: () => void }

export default function GroupsPanel({ onOpenConversation }: GroupsPanelProps) {
  const { user } = useAuth();
  const { groups, conversations, setActiveConversation } = useChat();
  const [query, setQuery] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [showApplications, setShowApplications] = useState(false);
  const [applications, setApplications] = useState<GroupApplication[]>([]);
  const [loadingApplications, setLoadingApplications] = useState(false);
  const [handlingID, setHandlingID] = useState("");
  const [applicationError, setApplicationError] = useState("");
  const filtered = groups.filter((item) => item.name.toLowerCase().includes(query.toLowerCase()));

  const loadApplications = useCallback(async () => {
    if (!user || groups.length === 0) {
      setApplications([]);
      return;
    }
    setLoadingApplications(true);
    setApplicationError("");
    const results = await Promise.allSettled(groups.map(async (group) => {
      const items = await IMSDK.getGroupApplicationListAsRecipient(group.groupId);
      return items.map((item) => ({ ...item, groupName: group.name }));
    }));
    setApplications(results.flatMap((result) => result.status === "fulfilled" ? result.value : []));
    setLoadingApplications(false);
  }, [groups, user]);

  useEffect(() => { void loadApplications(); }, [loadApplications]);

  const openGroup = (groupId: string) => {
    const conversationId = IMSDK.groupConversationId(groupId);
    const conversation = conversations.find((item) => item.conversationId === conversationId);
    if (conversation) {
      setActiveConversation(conversation.conversationId);
      onOpenConversation?.();
    }
  };

  const handleApplication = async (application: GroupApplication, accept: boolean) => {
    if (handlingID) return;
    setHandlingID(application.applicationId);
    setApplicationError("");
    try {
      if (accept) {
        await IMSDK.acceptGroupApplication(application);
      } else {
        await IMSDK.refuseGroupApplication(application);
      }
      setApplications((items) => items.filter((item) => item.applicationId !== application.applicationId));
    } catch (error) {
      setApplicationError(error instanceof Error ? error.message : "处理入群申请失败");
    } finally {
      setHandlingID("");
    }
  };

  return <div className="flex h-full w-full flex-col bg-surface-elevated">
    <header className="px-5 pb-3 pt-5">
      <div className="flex items-center justify-between">
        <div><p className="text-xs font-medium text-ink-muted">GROUPS</p><h1 className="mt-1 text-xl font-semibold text-ink">群组</h1></div>
        <button onClick={() => setShowCreate(true)} className="ui-press flex h-9 w-9 items-center justify-center rounded-control border border-edge text-ink-muted" title="创建群组"><Plus className="h-4 w-4" strokeWidth={1.75} /></button>
      </div>
      <div className="relative mt-4"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-muted" strokeWidth={1.75} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索群组" className="h-10 w-full rounded-control border border-edge bg-surface-muted pl-9 pr-3 text-sm outline-none focus:border-accent" /></div>
    </header>

    {!showApplications && applications.length > 0 && <div className="mx-5 mb-3 flex items-center justify-between rounded-control border border-accent/30 bg-accent-soft px-3 py-2.5">
      <div><p className="text-xs font-semibold text-accent">{applications.length} 条入群申请待处理</p><p className="mt-0.5 text-[11px] text-accent/80">{new Set(applications.map((item) => item.groupId)).size} 个群组</p></div>
      <button onClick={() => setShowApplications(true)} className="text-xs font-medium text-accent">处理</button>
    </div>}

    {showApplications ? <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-5">
      <button onClick={() => setShowApplications(false)} className="mb-4 flex items-center gap-1 text-xs font-medium text-ink-muted"><ArrowLeft className="h-3.5 w-3.5" strokeWidth={1.75} />返回群组</button>
      <div className="flex items-center justify-between"><h2 className="text-sm font-semibold text-ink">入群申请</h2><button onClick={() => void loadApplications()} className="text-xs text-ink-muted">刷新</button></div>
      {applicationError && <p className="mt-3 rounded-control bg-danger-soft px-3 py-2 text-xs text-danger">{applicationError}</p>}
      {loadingApplications ? <div className="flex justify-center py-16"><Loader2 className="h-5 w-5 animate-spin text-ink-muted" strokeWidth={1.75} /></div> : applications.length === 0 ? <div className="py-16 text-center text-sm text-ink-muted">暂无待处理申请</div> : <div className="mt-3 divide-y divide-edge">
        {applications.map((application) => <div key={application.applicationId} className="py-4">
          <div className="flex items-start gap-3"><UserAvatar src={application.user?.avatar} name={application.user?.displayName || application.userId} size="md" /><div className="min-w-0 flex-1">
            <div className="flex items-center justify-between gap-2"><p className="truncate text-sm font-medium text-ink">{application.user?.displayName || application.userId}</p><span className="flex-none text-[11px] text-ink-muted">{formatConvTime(application.createdAt)}</span></div>
            <p className="mt-0.5 text-xs text-ink-muted">申请加入「{application.groupName || application.groupId}」</p>
            {application.message && <p className="mt-3 rounded-control bg-surface-muted px-2.5 py-2 text-xs leading-5 text-ink-muted">{application.message}</p>}
            <div className="mt-3 flex gap-2"><button onClick={() => void handleApplication(application, true)} disabled={handlingID === application.applicationId} className="ui-press flex h-8 items-center gap-1.5 rounded-control bg-accent px-3 text-xs font-medium text-accent-fg hover:bg-accent-hover disabled:opacity-60">{handlingID === application.applicationId ? <Loader2 className="h-3.5 w-3.5 animate-spin" strokeWidth={1.75} /> : <Check className="h-3.5 w-3.5" strokeWidth={1.75} />}同意</button><button onClick={() => void handleApplication(application, false)} disabled={handlingID === application.applicationId} className="ui-press flex h-8 items-center gap-1.5 rounded-control border border-edge px-3 text-xs text-ink-muted disabled:opacity-60"><X className="h-3.5 w-3.5" strokeWidth={1.75} />拒绝</button></div>
          </div></div>
        </div>)}
      </div>}
    </div> : <div className="min-h-0 flex-1 overflow-y-auto">
      <p className="px-5 py-2 text-[11px] font-semibold uppercase text-ink-muted">我加入的群组 · {groups.length}</p>
      {filtered.length === 0 && <p className="px-5 py-10 text-center text-sm text-ink-muted">暂无群组</p>}
      {filtered.map((group) => <button key={group.groupId} onClick={() => openGroup(group.groupId)} className="flex w-full items-center gap-3 px-5 py-3.5 text-left hover:bg-surface-muted"><div className="flex h-11 w-11 items-center justify-center rounded-control bg-accent-soft text-accent"><UsersRound className="h-5 w-5" strokeWidth={1.75} /></div><div className="min-w-0 flex-1"><div className="flex items-center gap-1.5"><p className="truncate text-sm font-medium text-ink">{group.name}</p>{group.ownerId === user?.userId && <ShieldCheck className="h-3.5 w-3.5 text-amber-500" strokeWidth={1.75} />}{group.isMuted && <BellOff className="h-3.5 w-3.5 text-ink-muted" strokeWidth={1.75} />}</div><p className="mt-0.5 truncate text-xs text-ink-muted">{group.memberCount} 名成员{(group.notification || group.introduction) ? ` · ${group.notification || group.introduction}` : ""}</p></div><ChevronRight className="h-4 w-4 text-ink-muted/40" strokeWidth={1.75} /></button>)}
    </div>}
    {showCreate && <CreateGroupDialog onClose={() => setShowCreate(false)} />}
  </div>;
}
