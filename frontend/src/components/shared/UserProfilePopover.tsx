"use client";

// ============================================================
// UserProfilePopover — 个人信息弹窗，支持一键复制 UserID
// ============================================================
import React, { useEffect, useRef, useState, useCallback } from "react";
import { X, Copy, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import UserAvatar from "./UserAvatar";
import type { User } from "@/types";

interface UserProfilePopoverProps {
  user: User;
  onClose: () => void;
}

export default function UserProfilePopover({ user, onClose }: UserProfilePopoverProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const [copied, setCopied] = useState(false);

  // 点击外部关闭
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [onClose]);

  // Escape 关闭
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [onClose]);

  const handleCopyID = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(user.userId);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // 降级方案
      const ta = document.createElement("textarea");
      ta.value = user.userId;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [user.userId]);

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleDateString("zh-CN", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center pt-20">
      {/* 半透明遮罩 */}
      <div className="absolute inset-0 bg-black/20" onClick={onClose} />

      {/* 弹窗面板 */}
      <div
        ref={panelRef}
        className="relative w-80 bg-white rounded-2xl shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-200"
      >
        {/* 关闭按钮 */}
        <button
          onClick={onClose}
          className="absolute top-3 right-3 p-1 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X className="w-4 h-4" />
        </button>

        {/* 头像区 */}
        <div className="flex flex-col items-center pt-8 pb-4 bg-gradient-to-b from-indigo-50 to-white">
          <UserAvatar
            name={user.displayName || user.username || ""}
            size="xl"
            className="ring-4 ring-white shadow-lg"
          />
          <h3 className="mt-3 text-lg font-semibold text-gray-900">
            {user.displayName || user.username}
          </h3>
          {user.displayName && user.username && (
            <p className="text-sm text-gray-500">@{user.username}</p>
          )}
        </div>

        {/* 信息列表 */}
        <div className="px-5 pb-5 space-y-3">
          {/* 邮箱 */}
          {user.email && (
            <div>
              <label className="text-[11px] font-medium text-gray-400 uppercase tracking-wide">
                邮箱
              </label>
              <p className="text-sm text-gray-700 mt-0.5 break-all">{user.email}</p>
            </div>
          )}

          {/* UserID — 可复制 */}
          <div>
            <label className="text-[11px] font-medium text-gray-400 uppercase tracking-wide">
              User ID
            </label>
            <div className="flex items-center gap-2 mt-0.5">
              <code className="flex-1 text-xs bg-gray-100 rounded-lg px-2.5 py-1.5 text-gray-700 font-mono break-all select-all">
                {user.userId}
              </code>
              <button
                onClick={handleCopyID}
                className={cn(
                  "flex-shrink-0 p-1.5 rounded-lg transition-all duration-200",
                  copied
                    ? "bg-emerald-100 text-emerald-600"
                    : "bg-gray-100 text-gray-400 hover:bg-indigo-100 hover:text-indigo-600"
                )}
                title={copied ? "已复制" : "复制 UserID"}
              >
                {copied ? (
                  <Check className="w-4 h-4" />
                ) : (
                  <Copy className="w-4 h-4" />
                )}
              </button>
            </div>
          </div>

          {/* 注册时间 */}
          {user.createdAt && (
            <div>
              <label className="text-[11px] font-medium text-gray-400 uppercase tracking-wide">
                注册时间
              </label>
              <p className="text-sm text-gray-700 mt-0.5">{formatDate(user.createdAt)}</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
