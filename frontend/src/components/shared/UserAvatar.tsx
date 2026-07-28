"use client";

// ============================================================
// UserAvatar — 用户头像组件（首字母 Fallback）
// ============================================================
import React from "react";
import { cn } from "@/lib/utils";

interface UserAvatarProps {
  src?: string;
  name: string;
  size?: "sm" | "md" | "lg" | "xl";
  className?: string;
}

const sizeMap = {
  sm: "w-8 h-8 text-xs",
  md: "w-10 h-10 text-sm",
  lg: "w-12 h-12 text-base",
  xl: "w-16 h-16 text-xl",
};

const colors = [
  "bg-blue-500", "bg-emerald-500", "bg-violet-500",
  "bg-amber-500", "bg-rose-500", "bg-cyan-500",
  "bg-indigo-500", "bg-teal-500",
];

function getColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) {
    hash = name.charCodeAt(i) + ((hash << 5) - hash);
  }
  return colors[Math.abs(hash) % colors.length];
}

function getInitials(name: string): string {
  return name.slice(0, 2).toUpperCase();
}

export default function UserAvatar({
  src,
  name,
  size = "md",
  className,
}: UserAvatarProps) {
  const sizeClass = sizeMap[size];

  if (src) {
    return (
      <img
        src={src}
        alt={name}
        className={cn(sizeClass, "rounded-full object-cover flex-shrink-0", className)}
        onError={(e) => {
          // Fallback: 隐藏图片，显示文字头像
          (e.target as HTMLImageElement).style.display = "none";
          const parent = (e.target as HTMLImageElement).parentElement;
          if (parent) {
            const fallback = parent.querySelector("[data-fallback]") as HTMLElement;
            if (fallback) fallback.style.display = "flex";
          }
        }}
      />
    );
  }

  return (
    <div
      className={cn(
        sizeClass,
        getColor(name),
        "rounded-full flex items-center justify-center text-white font-semibold flex-shrink-0 select-none",
        className
      )}
    >
      {getInitials(name)}
    </div>
  );
}
