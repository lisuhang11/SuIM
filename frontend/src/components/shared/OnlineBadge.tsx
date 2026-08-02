"use client";

// ============================================================
// OnlineBadge — 在线状态指示器
// ============================================================
import React from "react";
import type { UserStatus } from "@/types";
import { cn } from "@/lib/utils";

interface OnlineBadgeProps {
  status: UserStatus;
  size?: "sm" | "md" | "lg";
  className?: string;
}

const sizeMap = {
  sm: "w-2 h-2",
  md: "w-2.5 h-2.5",
  lg: "w-3 h-3",
};

const statusColorMap: Record<UserStatus, string> = {
  online: "bg-accent",
  away: "bg-amber-400",
  busy: "bg-danger",
  offline: "bg-ink-muted/40",
};

export default function OnlineBadge({ status, size = "md", className }: OnlineBadgeProps) {
  const sizeClass = sizeMap[size];

  return (
    <span
      className={cn(
        sizeClass,
        statusColorMap[status],
        "rounded-full border-2 border-surface-elevated flex-shrink-0",
        className
      )}
      title={status}
    />
  );
}
