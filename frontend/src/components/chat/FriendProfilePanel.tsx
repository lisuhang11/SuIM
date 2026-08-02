"use client";

import React, { useEffect, useState } from "react";
import { ArrowLeft, Loader2, MessageCircle, X } from "lucide-react";
import { IMSDK } from "@/suim-sdk";
import type { Contact } from "@/types";
import UserAvatar from "../shared/UserAvatar";

interface FriendProfilePanelProps {
  contact: Contact;
  onClose: () => void;
  onUpdated: () => Promise<void> | void;
  onMessage: (userId: string) => void;
}

export default function FriendProfilePanel({
  contact,
  onClose,
  onUpdated,
  onMessage,
}: FriendProfilePanelProps) {
  const [remark, setRemark] = useState(contact.remark ?? "");
  const [savingRemark, setSavingRemark] = useState(false);
  const [error, setError] = useState("");
  const [editingRemark, setEditingRemark] = useState(false);

  useEffect(() => {
    setRemark(contact.remark ?? "");
    setError("");
    setEditingRemark(false);
  }, [contact]);

  const nickname = contact.nickname || contact.displayName;
  const title = contact.remark || nickname;

  const saveRemark = async () => {
    if (savingRemark) return;
    setSavingRemark(true);
    setError("");
    try {
      await IMSDK.updateFriend(contact.userId, { remark });
      await onUpdated();
      setEditingRemark(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存备注失败");
    } finally {
      setSavingRemark(false);
    }
  };

  return (
    <div className="absolute inset-0 z-40 flex items-center justify-center bg-ink/35 p-4">
      <div className="flex h-[min(560px,calc(100dvh-48px))] w-full max-w-sm flex-col overflow-hidden rounded-control bg-surface-elevated shadow-panel">
        <div className="flex h-14 flex-none items-center justify-between border-b border-edge px-4">
          <button
            onClick={onClose}
            className="ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted"
            title="返回"
          >
            <ArrowLeft className="h-4 w-4" strokeWidth={1.75} />
          </button>
          <h2 className="text-sm font-semibold text-ink">好友资料</h2>
          <button
            onClick={onClose}
            className="ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted"
            title="关闭"
          >
            <X className="h-4 w-4" strokeWidth={1.75} />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-6">
          <div className="flex flex-col items-center text-center">
            <UserAvatar src={contact.avatar} name={title} size="xl" className="h-16 w-16 text-xl" />
            <p className="mt-3 text-base font-semibold text-ink">{title}</p>
            <p className="mt-1 text-xs text-ink-muted">
              昵称：{nickname}
              {contact.username ? ` · @${contact.username}` : null}
            </p>
          </div>

          <div className="mt-6 overflow-hidden rounded-control border border-edge">
            <div className="px-4 py-3">
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm text-ink">备注</span>
                {!editingRemark ? (
                  <button
                    type="button"
                    onClick={() => setEditingRemark(true)}
                    className="max-w-[60%] truncate text-sm text-ink-muted hover:text-accent"
                  >
                    {remark || "未设置"} ›
                  </button>
                ) : null}
              </div>
              {editingRemark ? (
                <div className="mt-3 flex gap-2">
                  <input
                    autoFocus
                    value={remark}
                    maxLength={64}
                    onChange={(e) => setRemark(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void saveRemark();
                    }}
                    placeholder="输入备注"
                    className="h-9 min-w-0 flex-1 rounded-control border border-edge px-3 text-sm outline-none focus:border-accent"
                  />
                  <button
                    type="button"
                    disabled={savingRemark}
                    onClick={() => void saveRemark()}
                    className="ui-press h-9 rounded-control bg-accent px-3 text-sm font-medium text-accent-fg hover:bg-accent-hover disabled:opacity-50"
                  >
                    {savingRemark ? <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} /> : "保存"}
                  </button>
                </div>
              ) : null}
            </div>
          </div>

          {error ? <p className="mt-3 text-xs text-danger">{error}</p> : null}

          <button
            type="button"
            onClick={() => onMessage(contact.userId)}
            className="ui-press mt-6 flex h-11 w-full items-center justify-center gap-2 rounded-control bg-accent text-sm font-medium text-accent-fg hover:bg-accent-hover"
          >
            <MessageCircle className="h-4 w-4" strokeWidth={1.75} />
            发消息
          </button>
        </div>
      </div>
    </div>
  );
}
