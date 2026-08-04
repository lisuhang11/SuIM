"use client";

// ============================================================
// CreateGroupDialog — 创建群聊对话框
// ============================================================
import React, { useState } from "react";
import { X, Search, Check } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import type { Contact } from "@/types";
import UserAvatar from "../shared/UserAvatar";
import { cn } from "@/lib/utils";

interface CreateGroupDialogProps {
  onClose: () => void;
}

export default function CreateGroupDialog({ onClose }: CreateGroupDialogProps) {
  const { contacts, createGroup, setActiveConversation } = useChat();
  const [name, setName] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");

  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  const filtered = contacts.filter(
    (c) =>
      c.displayName.toLowerCase().includes(search.toLowerCase()) ||
      c.username.toLowerCase().includes(search.toLowerCase())
  );

  const toggle = (id: string) => {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  };

  const handleCreate = async () => {
    if (!name.trim() || selected.size === 0 || creating) return;
    setCreating(true);
    setError("");
    try {
      const conv = await createGroup(name.trim(), Array.from(selected));
      if (conv) {
        setActiveConversation(conv.conversationId);
        onClose();
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建群组失败");
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40">
      <div className="bg-surface-elevated rounded-control shadow-panel w-full max-w-md mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-edge">
          <h3 className="font-semibold text-ink">创建群聊</h3>
          <button
            onClick={onClose}
            className="ui-press p-1 rounded-control hover:bg-surface-muted text-ink-muted"
          >
            <X className="w-5 h-5" strokeWidth={1.75} />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 flex-1 overflow-y-auto">
          {/* 群名 */}
          <div>
            <label className="block text-sm font-medium text-ink mb-1.5">
              群聊名称
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入群聊名称"
              className="w-full px-4 py-2.5 rounded-control border border-edge text-sm
                focus:outline-none focus:border-accent focus:ring-2 focus:ring-accent/20"
            />
          </div>

          {/* 已选成员 */}
          {selected.size > 0 && (
            <div>
              <label className="block text-sm font-medium text-ink mb-1.5">
                已选 {selected.size} 人
              </label>
              <div className="flex flex-wrap gap-1.5">
                {Array.from(selected).map((id) => {
                  const c = contacts.find((x) => x.userId === id);
                  return c ? (
                    <span
                      key={id}
                      className="flex items-center gap-1 bg-accent-soft text-accent px-2 py-1 rounded-control text-xs"
                    >
                      {c.displayName}
                      <button onClick={() => toggle(id)} className="hover:text-danger">
                        <X className="w-3 h-3" strokeWidth={1.75} />
                      </button>
                    </span>
                  ) : null;
                })}
              </div>
            </div>
          )}

          {/* 搜索 */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-muted" strokeWidth={1.75} />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="搜索联系人..."
              className="w-full pl-9 pr-4 py-2 rounded-control border border-edge text-sm
                focus:outline-none focus:border-accent focus:ring-2 focus:ring-accent/20"
            />
          </div>

          {/* 联系人列表 */}
          <div className="space-y-0.5 max-h-48 overflow-y-auto -mx-2">
            {filtered.map((contact) => (
              <button
                key={contact.userId}
                onClick={() => toggle(contact.userId)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2 rounded-control text-left transition-colors",
                  selected.has(contact.userId)
                    ? "bg-accent-soft"
                    : "hover:bg-surface-muted"
                )}
              >
                <UserAvatar name={contact.displayName} size="md" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-ink truncate">
                    {contact.displayName}
                  </p>
                  <p className="text-xs text-ink-muted">@{contact.username}</p>
                </div>
                <div
                  className={cn(
                    "w-5 h-5 rounded-control border-2 flex items-center justify-center transition-colors",
                    selected.has(contact.userId)
                      ? "bg-accent border-accent"
                      : "border-edge"
                  )}
                >
                  {selected.has(contact.userId) && (
                    <Check className="w-3 h-3 text-accent-fg" strokeWidth={1.75} />
                  )}
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Footer */}
        <div className="px-5 py-4 border-t border-edge space-y-3">
          {error && <p className="text-sm text-danger">{error}</p>}
          <div className="flex gap-3">
          <button
            type="button"
            onClick={onClose}
            className="ui-press flex-1 py-2.5 rounded-control border border-edge text-sm font-medium text-ink-muted hover:bg-surface-muted transition-colors"
          >
            取消
          </button>
          <button
            type="button"
            onClick={() => void handleCreate()}
            disabled={!name.trim() || selected.size === 0 || creating}
            className={cn(
              "ui-press flex-1 py-2.5 rounded-control text-sm font-medium transition-all",
              name.trim() && selected.size > 0 && !creating
                ? "bg-accent text-accent-fg hover:bg-accent-hover active:scale-[0.98]"
                : "bg-surface-muted text-ink-muted/40 cursor-not-allowed"
            )}
          >
            {creating ? "创建中…" : "创建群聊"}
          </button>
          </div>
        </div>
      </div>
    </div>
  );
}
