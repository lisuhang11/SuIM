"use client";

import React, { useMemo, useState } from "react";
import { MessageCircle, Search, UserPlus, X } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import type { Contact } from "@/types";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import AddFriendPanel from "./AddFriendPanel";
import FriendRequestsPanel from "./FriendRequestsPanel";
import FriendProfilePanel from "./FriendProfilePanel";
import BlacklistPanel from "./BlacklistPanel";

interface FriendsPanelProps { onOpenConversation?: () => void }

export default function FriendsPanel({ onOpenConversation }: FriendsPanelProps) {
  const { contacts, friendRequestBadge, openOrCreatePrivateChat, refreshContacts } = useChat();
  const [tab, setTab] = useState<"contacts" | "requests" | "blacklist">("contacts");
  const [query, setQuery] = useState("");
  const [showAddFriend, setShowAddFriend] = useState(false);
  const [profileContact, setProfileContact] = useState<Contact | null>(null);
  const [openingId, setOpeningId] = useState<string | null>(null);
  const filtered = useMemo(
    () =>
      contacts.filter((item) =>
        `${item.remark ?? ""}${item.displayName}${item.nickname ?? ""}${item.username}`
          .toLowerCase()
          .includes(query.toLowerCase())
      ),
    [contacts, query]
  );

  const openChat = async (userId: string) => {
    if (openingId) return;
    setOpeningId(userId);
    try {
      const id = await openOrCreatePrivateChat(userId);
      if (id) onOpenConversation?.();
    } finally {
      setOpeningId(null);
    }
  };

  return (
    <div className="relative flex h-full w-full flex-col bg-surface-elevated">
      <header className="px-5 pb-3 pt-5">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs font-medium text-ink-muted">RELATIONS</p>
            <h1 className="mt-1 text-xl font-semibold text-ink">通讯录</h1>
          </div>
          <button
            onClick={() => setShowAddFriend(true)}
            className="ui-press flex h-9 w-9 items-center justify-center rounded-control border border-edge text-ink-muted"
            title="添加好友"
          >
            <UserPlus className="h-4 w-4" strokeWidth={1.75} />
          </button>
        </div>
        <div className="mt-4 flex rounded-control bg-surface-muted p-1">
          <button
            onClick={() => setTab("contacts")}
            className={cn(
              "ui-press h-8 flex-1 rounded-[6px] text-xs font-medium",
              tab === "contacts" ? "bg-surface-elevated text-ink shadow-sm" : "text-ink-muted"
            )}
          >
            好友 {contacts.length}
          </button>
          <button
            onClick={() => setTab("requests")}
            className={cn(
              "ui-press h-8 flex-1 rounded-[6px] text-xs font-medium",
              tab === "requests" ? "bg-surface-elevated text-ink shadow-sm" : "text-ink-muted"
            )}
          >
            新的朋友{" "}
            {friendRequestBadge > 0 && (
              <span className="ml-1 rounded-control bg-danger px-1.5 text-[10px] text-white">
                {friendRequestBadge > 99 ? "99+" : friendRequestBadge}
              </span>
            )}
          </button>
          <button
            onClick={() => setTab("blacklist")}
            className={cn(
              "ui-press h-8 flex-1 rounded-[6px] text-xs font-medium",
              tab === "blacklist" ? "bg-surface-elevated text-ink shadow-sm" : "text-ink-muted"
            )}
          >
            黑名单
          </button>
        </div>
      </header>

      {tab === "contacts" ? (
        <>
          <div className="px-5 pb-3">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-muted" strokeWidth={1.75} />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="搜索好友"
                className="h-10 w-full rounded-control border border-edge bg-surface-muted pl-9 pr-3 text-sm outline-none focus:border-accent"
              />
            </div>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto">
            <p className="px-5 py-2 text-[11px] font-semibold uppercase text-ink-muted">联系人</p>
            {filtered.map((contact) => (
              <div
                key={contact.userId}
                role="button"
                tabIndex={0}
                onClick={() => setProfileContact(contact)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") setProfileContact(contact);
                }}
                className="group flex cursor-pointer items-center gap-3 px-5 py-3 hover:bg-surface-muted"
              >
                <div className="relative">
                  <UserAvatar src={contact.avatar} name={contact.displayName} size="md" />
                  <span
                    className={cn(
                      "absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-surface-elevated",
                      contact.status === "online"
                        ? "bg-accent"
                        : contact.status === "away"
                          ? "bg-amber-400"
                          : "bg-ink-muted/40"
                    )}
                  />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-ink">{contact.displayName}</p>
                  <p className="truncate text-xs text-ink-muted">
                    @{contact.username} ·{" "}
                    {contact.status === "online" ? "在线" : contact.status === "away" ? "离开" : "离线"}
                  </p>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    void openChat(contact.userId);
                  }}
                  disabled={openingId === contact.userId}
                  className="ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted opacity-100 hover:bg-accent-soft hover:text-accent sm:opacity-0 sm:group-hover:opacity-100 disabled:opacity-40"
                  title="发消息"
                  aria-label={`给 ${contact.displayName} 发消息`}
                >
                  <MessageCircle className="h-4 w-4" strokeWidth={1.75} />
                </button>
              </div>
            ))}
          </div>
        </>
      ) : tab === "requests" ? (
        <div className="min-h-0 flex-1">
          <FriendRequestsPanel />
        </div>
      ) : (
        <div className="min-h-0 flex-1">
          <BlacklistPanel />
        </div>
      )}

      {showAddFriend && (
        <div className="absolute inset-0 z-40 flex items-center justify-center bg-ink/35 p-4">
          <div className="flex h-[min(560px,calc(100dvh-48px))] w-full max-w-sm flex-col overflow-hidden rounded-control bg-surface-elevated shadow-panel">
            <div className="flex h-14 flex-none items-center justify-between border-b border-edge px-4">
              <h2 className="text-sm font-semibold text-ink">添加好友</h2>
              <button
                onClick={() => setShowAddFriend(false)}
                className="ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted"
                title="关闭"
              >
                <X className="h-4 w-4" strokeWidth={1.75} />
              </button>
            </div>
            <div className="min-h-0 flex-1">
              <AddFriendPanel embedded />
            </div>
          </div>
        </div>
      )}

      {profileContact ? (
        <FriendProfilePanel
          contact={contacts.find((c) => c.userId === profileContact.userId) ?? profileContact}
          onClose={() => setProfileContact(null)}
          onUpdated={refreshContacts}
          onMessage={(userId) => {
            setProfileContact(null);
            void openChat(userId);
          }}
        />
      ) : null}
    </div>
  );
}
