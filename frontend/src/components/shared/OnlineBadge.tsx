"use client";

// ============================================================
// OnlineBadge — 在线状态指示器
// ============================================================
import React from "react";
import type { UserStatus } from "@/types";
import { getStatusColor } from "@/data/mock";
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

export default function OnlineBadge({ status, size = "md", className }: OnlineBadgeProps) {
  const color = getStatusColor(status);
  const sizeClass = sizeMap[size];

  return (
    <span
      className={cn(
        sizeClass,
        "rounded-full border-2 border-white dark:border-gray-800 flex-shrink-0",
        className
      )}
      style={{ backgroundColor: color }}
      title={status}
    />
  );
}
