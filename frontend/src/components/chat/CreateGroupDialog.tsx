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
    if (!name.trim() || selected.size === 0) return;
    const conv = await createGroup(name.trim(), Array.from(selected));
    if (conv) {
      setActiveConversation(conv.conversationId);
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 max-h-[80vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-100">
          <h3 className="font-semibold text-gray-900">创建群聊</h3>
          <button
            onClick={onClose}
            className="p-1 rounded-lg hover:bg-gray-100 text-gray-400"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-5 space-y-4 flex-1 overflow-y-auto">
          {/* 群名 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">
              群聊名称
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="输入群聊名称"
              className="w-full px-4 py-2.5 rounded-xl border border-gray-200 text-sm
                focus:outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            />
          </div>

          {/* 已选成员 */}
          {selected.size > 0 && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1.5">
                已选 {selected.size} 人
              </label>
              <div className="flex flex-wrap gap-1.5">
                {Array.from(selected).map((id) => {
                  const c = contacts.find((x) => x.userId === id);
                  return c ? (
                    <span
                      key={id}
                      className="flex items-center gap-1 bg-indigo-50 text-indigo-600 px-2 py-1 rounded-lg text-xs"
                    >
                      {c.displayName}
                      <button onClick={() => toggle(id)} className="hover:text-red-500">
                        <X className="w-3 h-3" />
                      </button>
                    </span>
                  ) : null;
                })}
              </div>
            </div>
          )}

          {/* 搜索 */}
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

          {/* 联系人列表 */}
          <div className="space-y-0.5 max-h-48 overflow-y-auto -mx-2">
            {filtered.map((contact) => (
              <button
                key={contact.userId}
                onClick={() => toggle(contact.userId)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2 rounded-xl text-left transition-colors",
                  selected.has(contact.userId)
                    ? "bg-indigo-50"
                    : "hover:bg-gray-50"
                )}
              >
                <UserAvatar name={contact.displayName} size="md" />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-gray-900 truncate">
                    {contact.displayName}
                  </p>
                  <p className="text-xs text-gray-400">@{contact.username}</p>
                </div>
                <div
                  className={cn(
                    "w-5 h-5 rounded-full border-2 flex items-center justify-center transition-colors",
                    selected.has(contact.userId)
                      ? "bg-indigo-500 border-indigo-500"
                      : "border-gray-300"
                  )}
                >
                  {selected.has(contact.userId) && (
                    <Check className="w-3 h-3 text-white" />
                  )}
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
            disabled={!name.trim() || selected.size === 0}
            className={cn(
              "flex-1 py-2.5 rounded-xl text-sm font-medium text-white transition-all",
              name.trim() && selected.size > 0
                ? "bg-indigo-500 hover:bg-indigo-600 active:scale-[0.98] shadow-md shadow-indigo-200"
                : "bg-gray-300 cursor-not-allowed"
            )}
          >
            创建群聊
          </button>
        </div>
      </div>
    </div>
  );
}
