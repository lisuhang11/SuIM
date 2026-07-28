"use client";

// ============================================================
// NotificationsPanel — 通知面板
// ============================================================
import React from "react";
import { Bell, AtSign, UserPlus, Users, Info, CheckCheck } from "lucide-react";
import { useChat } from "@/contexts/ChatContext";
import { formatTime } from "@/lib/utils";
import { cn } from "@/lib/utils";
import type { Notification, NotificationType } from "@/types";

const typeConfig: Record<NotificationType, { icon: React.ReactNode; color: string; bg: string }> = {
  friend_request: {
    icon: <UserPlus className="w-4 h-4" />,
    color: "text-green-500",
    bg: "bg-green-50",
  },
  group_invite: {
    icon: <Users className="w-4 h-4" />,
    color: "text-blue-500",
    bg: "bg-blue-50",
  },
  mention: {
    icon: <AtSign className="w-4 h-4" />,
    color: "text-amber-500",
    bg: "bg-amber-50",
  },
  system: {
    icon: <Info className="w-4 h-4" />,
    color: "text-gray-400",
    bg: "bg-gray-50",
  },
};

interface NotificationsPanelProps {
  onClose?: () => void;
}

export default function NotificationsPanel({ onClose }: NotificationsPanelProps) {
  const {
    notifications,
    activeConversationId,
    setActiveConversation,
    markNotificationRead,
    markAllNotificationsRead,
  } = useChat();

  const sorted = [...(notifications || [])].sort(
    (a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  );

  const handleClick = (notif: Notification) => {
    markNotificationRead(notif.notificationId);
    // 如果有关联的会话ID，跳转到该会话
    if (notif.refId && notif.type !== "system") {
      setActiveConversation(notif.refId);
    }
    onClose?.();
  };

  return (
    <div className="h-full flex flex-col bg-white">
      {/* 标题栏 */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200">
        <h3 className="font-semibold text-gray-900 text-sm">通知</h3>
        <button
          onClick={markAllNotificationsRead}
          className="flex items-center gap-1 text-xs text-indigo-500 hover:text-indigo-600
            transition-colors font-medium"
        >
          <CheckCheck className="w-3.5 h-3.5" />
          全部已读
        </button>
      </div>

      {/* 通知列表 */}
      <div className="flex-1 overflow-y-auto">
        {sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-400 text-sm py-20">
            <Bell className="w-8 h-8 mb-2 opacity-40" />
            <p>暂无通知</p>
          </div>
        ) : (
          sorted.map((notif) => {
            const config = typeConfig[notif.type];
            return (
              <button
                key={notif.notificationId}
                onClick={() => handleClick(notif)}
                className={cn(
                  "w-full flex items-start gap-3 px-4 py-3 text-left transition-colors",
                  "hover:bg-gray-50",
                  !notif.isRead && "bg-indigo-50/40 hover:bg-indigo-50/60"
                )}
              >
                {/* 图标 */}
                <div
                  className={cn(
                    "w-9 h-9 rounded-xl flex items-center justify-center flex-shrink-0 mt-0.5",
                    config.bg, config.color
                  )}
                >
                  {config.icon}
                </div>

                {/* 内容 */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-2">
                    <h4
                      className={cn(
                        "text-sm truncate",
                        !notif.isRead ? "font-semibold text-gray-900" : "font-medium text-gray-700"
                      )}
                    >
                      {notif.title}
                    </h4>
                    <div className="flex items-center gap-1.5 flex-shrink-0">
                      {!notif.isRead && (
                        <span className="w-2 h-2 rounded-full bg-indigo-500" />
                      )}
                      <span className="text-[11px] text-gray-400">
                        {formatTime(notif.createdAt)}
                      </span>
                    </div>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5 line-clamp-2">
                    {notif.content}
                  </p>
                </div>
              </button>
            );
          })
        )}
      </div>
    </div>
  );
}
