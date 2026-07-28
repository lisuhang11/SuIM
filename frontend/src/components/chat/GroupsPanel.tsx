"use client";

// ============================================================
// GroupsPanel — 群聊面板（一级菜单）
// 创建群聊 → 向右弹出大窗，左列选人 / 右列已选
// ============================================================
import React, { useState, useMemo } from "react";
import { UsersRound, Plus, Check, Search, X, ChevronLeft } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import OnlineBadge from "../shared/OnlineBadge";
import { getStatusText } from "@/data/mock";

export default function GroupsPanel({ panelOpen, onPanelToggle }: {
  panelOpen: boolean;
  onPanelToggle: (open: boolean) => void;
}) {
  const { groups, contacts, createGroup } = useChat();
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [groupName, setGroupName] = useState("");
  const [searchFilter, setSearchFilter] = useState("");

  const filteredContacts = useMemo(() => {
    const sel = Array.from(selectedIds);
    const unselected = contacts.filter((c) => !selectedIds.has(c.userId));
    if (!searchFilter.trim()) return unselected;
    const q = searchFilter.toLowerCase();
    return unselected.filter(
      (c) => c.displayName.toLowerCase().includes(q) || c.username.toLowerCase().includes(q)
    );
  }, [contacts, searchFilter, selectedIds]);

  const selectedContacts = useMemo(
    () => contacts.filter((c) => selectedIds.has(c.userId)),
    [contacts, selectedIds]
  );

  const toggleContact = (userId: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(userId)) next.delete(userId);
      else next.add(userId);
      return next;
    });
  };

  const handleCreate = async () => {
    if (selectedIds.size === 0) return;
    const name = groupName.trim() || `新的群聊 (${selectedIds.size + 1}人)`;
    await createGroup(name, Array.from(selectedIds));
    // 重置
    onPanelToggle(false);
    setSelectedIds(new Set());
    setGroupName("");
    setSearchFilter("");
  };

  const handleClose = () => {
    onPanelToggle(false);
    // 保留已选以便重新打开时继续编辑
  };

  return (
    <div className="flex flex-col h-full relative">
      {/* ====== 创建群聊按钮 ====== */}
      <div className="flex-shrink-0 px-3 pt-3 pb-2">
        <button
          onClick={() => onPanelToggle(true)}
          className={cn(
            "w-full flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-medium",
            "bg-gray-100 text-gray-700 hover:bg-gray-200",
            "transition-all duration-200 ease-out"
          )}
        >
          <div className="w-9 h-9 rounded-lg bg-indigo-100 flex items-center justify-center flex-shrink-0">
            <Plus className="w-4 h-4 text-indigo-500" />
          </div>
          <span>创建群聊</span>
        </button>
      </div>

      {/* ====== 已加入群聊列表 ====== */}
      <div className="flex-1 overflow-y-auto">
        {groups.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-gray-400 text-sm">
            <UsersRound className="w-10 h-10 mb-3 opacity-30" />
            <p className="font-medium">暂无群聊</p>
          </div>
        ) : (
          groups.map((group) => (
            <button
              key={group.groupId}
              className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 transition-colors duration-150 text-left"
            >
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-400 to-purple-500 flex items-center justify-center flex-shrink-0 text-white text-sm font-bold">
                {group.name.slice(0, 2).toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <h4 className="text-sm font-medium text-gray-900 truncate">{group.name}</h4>
                <p className="text-xs text-gray-400">{group.memberCount} 名成员</p>
              </div>
            </button>
          ))
        )}
      </div>

      {/* ====== 向右弹出创建面板 ====== */}
      {/* 遮罩层 */}
      <div
        className={cn(
          "fixed inset-0 z-40 transition-colors duration-250",
          panelOpen ? "bg-black/20 pointer-events-auto" : "bg-transparent pointer-events-none"
        )}
        onClick={handleClose}
      />

      {/* 弹出面板 — 外层裁剪容器防越界 */}
      <div className={cn(
        "fixed top-0 bottom-0 left-[68px] z-40 overflow-hidden",
        panelOpen ? "pointer-events-auto" : "pointer-events-none"
      )}
        style={{ width: '520px' }}>
        <div
          className={cn(
            "w-full h-full flex flex-col bg-white",
            "transition-transform duration-300 ease-in-out",
            panelOpen ? "translate-x-0 shadow-2xl shadow-black/10" : "-translate-x-full shadow-none"
          )}
        >
          {/* 面板内容略——头部 */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-100 flex-shrink-0">
          <div>
            <h2 className="text-base font-semibold text-gray-900">创建群聊</h2>
            <p className="text-xs text-gray-400 mt-0.5">选择联系人并设置群名</p>
          </div>
          <button
            onClick={handleClose}
            className="p-1.5 rounded-lg text-gray-400 hover:text-gray-600 hover:bg-gray-100 transition-colors"
          >
            <ChevronLeft className="w-5 h-5" />
          </button>
        </div>

        {/* 主体 — 双列 */}
        <div className="flex-1 flex min-h-0">
          {/* ====== 左列：可选联系人 ====== */}
          <div className="w-[55%] flex flex-col border-r border-gray-100">
            {/* 搜索 */}
            <div className="px-3 py-2.5 flex-shrink-0">
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-gray-400" />
                <input
                  type="text"
                  value={searchFilter}
                  onChange={(e) => setSearchFilter(e.target.value)}
                  placeholder="搜索联系人..."
                  className="w-full pl-8 pr-3 py-2 text-xs bg-gray-50 border border-gray-200 rounded-lg
                    focus:bg-white focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 outline-none transition-all"
                />
              </div>
            </div>

            {/* 联系人列表 */}
            <div className="flex-1 overflow-y-auto">
              {filteredContacts.length === 0 && !searchFilter.trim() ? (
                <div className="flex items-center justify-center py-12 text-gray-400 text-xs">
                  没有更多联系人
                </div>
              ) : filteredContacts.length === 0 ? (
                <div className="flex items-center justify-center py-12 text-gray-400 text-xs">
                  未找到匹配的联系人
                </div>
              ) : (
                filteredContacts.map((contact) => {
                  const isSel = selectedIds.has(contact.userId);
                  return (
                    <button
                      key={contact.userId}
                      onClick={() => toggleContact(contact.userId)}
                      className={cn(
                        "w-full flex items-center gap-2.5 px-4 py-2.5 transition-colors duration-150 text-left",
                        "border-b border-gray-50 last:border-0",
                        isSel ? "bg-indigo-50" : "hover:bg-gray-50"
                      )}
                    >
                      <div className="relative flex-shrink-0">
                        <UserAvatar name={contact.displayName} size="sm" />
                        <OnlineBadge status={contact.status} size="sm" className="absolute -bottom-0.5 -right-0.5" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <h4 className="text-xs font-medium text-gray-900 truncate">{contact.displayName}</h4>
                        <p className="text-[10px] text-gray-400">@{contact.username} · {getStatusText(contact.status)}</p>
                      </div>
                      <div className={cn(
                        "w-5 h-5 rounded-md border-2 flex items-center justify-center flex-shrink-0 transition-all duration-200",
                        isSel ? "bg-indigo-500 border-indigo-500" : "border-gray-300"
                      )}>
                        {isSel && <Check className="w-3 h-3 text-white" />}
                      </div>
                    </button>
                  );
                })
              )}
            </div>
          </div>

          {/* ====== 右列：已选 + 创建 ====== */}
          <div className="flex-1 flex flex-col bg-gray-50/50">
            {/* 群名 */}
            <div className="px-3 pt-3 pb-2 flex-shrink-0">
              <input
                type="text"
                value={groupName}
                onChange={(e) => setGroupName(e.target.value)}
                placeholder="群聊名称（选填）"
                className="w-full px-3 py-2 text-xs bg-white border border-gray-200 rounded-lg
                  focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 outline-none transition-all"
              />
            </div>

            {/* 已选标题 */}
            <div className="px-3 py-1.5 flex items-center justify-between flex-shrink-0">
              <span className="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">
                已选 {selectedIds.size} 人
              </span>
              {selectedIds.size > 0 && (
                <button
                  onClick={() => setSelectedIds(new Set())}
                  className="text-[10px] text-gray-400 hover:text-red-500 transition-colors"
                >
                  清空
                </button>
              )}
            </div>

            {/* 已选联系人列表 */}
            <div className="flex-1 overflow-y-auto px-2">
              {selectedContacts.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-10 text-gray-400 text-xs">
                  <UsersRound className="w-6 h-6 mb-2 opacity-30" />
                  <p>从左侧选择联系人</p>
                </div>
              ) : (
                <div className="space-y-1 pb-2">
                  {selectedContacts.map((contact) => (
                    <div
                      key={contact.userId}
                      className="flex items-center gap-2 px-2 py-1.5 bg-white rounded-lg border border-gray-100 group"
                    >
                      <UserAvatar name={contact.displayName} size="sm" />
                      <span className="flex-1 text-xs font-medium text-gray-700 truncate">{contact.displayName}</span>
                      <button
                        onClick={() => toggleContact(contact.userId)}
                        className="p-0.5 rounded-full text-gray-300 hover:text-red-400 hover:bg-red-50 opacity-0 group-hover:opacity-100 transition-all"
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* 创建按钮 */}
            <div className="px-3 py-3 flex-shrink-0 border-t border-gray-100">
              <button
                onClick={handleCreate}
                disabled={selectedIds.size === 0}
                className={cn(
                  "w-full py-2.5 rounded-lg text-sm font-medium transition-all duration-200",
                  selectedIds.size > 0
                    ? "bg-indigo-500 text-white hover:bg-indigo-600 active:scale-[0.98] shadow-sm shadow-indigo-200"
                    : "bg-gray-200 text-gray-400 cursor-not-allowed"
                )}
              >
                创建群聊 ({selectedIds.size > 0 ? selectedIds.size + 1 : 0}人)
              </button>
            </div>
          </div>
        </div>
      </div>
      </div>
    </div>
  );
}
