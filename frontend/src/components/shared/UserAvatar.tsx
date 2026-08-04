"use client";

// UserAvatar — photo or initials fallback
import React, { useEffect, useState } from "react";
import { cn } from "@/lib/utils";
import { IMSDK } from "@/suim-sdk";

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
  "bg-accent",
  "bg-teal-600",
  "bg-cyan-600",
  "bg-amber-500",
  "bg-rose-500",
  "bg-blue-500",
  "bg-sky-600",
  "bg-zinc-600",
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
  const [failed, setFailed] = useState(false);
  const [resolvedSrc, setResolvedSrc] = useState("");

  useEffect(() => {
    let active = true;
    setFailed(false);

    if (!src) {
      setResolvedSrc("");
      return;
    }

    // Optimistic: show http(s)/blob immediately while API paths resolve
    if (
      src.startsWith("blob:") ||
      src.startsWith("data:") ||
      /^https?:\/\//i.test(src)
    ) {
      setResolvedSrc(src);
    }

    void IMSDK.resolveAvatarURL(src)
      .then((value) => {
        if (!active) return;
        setResolvedSrc(value || src);
      })
      .catch(() => {
        if (!active) return;
        // Keep direct http(s) src if resolve failed; otherwise mark failed
        if (!/^https?:\/\//i.test(src) && !src.startsWith("blob:") && !src.startsWith("data:")) {
          setFailed(true);
        }
      });

    return () => {
      active = false;
    };
  }, [src]);

  if (resolvedSrc && !failed) {
    return (
      <img
        src={resolvedSrc}
        alt={name}
        className={cn(sizeClass, "rounded-control object-cover flex-shrink-0", className)}
        onError={() => setFailed(true)}
      />
    );
  }

  return (
    <div
      className={cn(
        sizeClass,
        getColor(name),
        "rounded-control flex items-center justify-center text-white font-semibold flex-shrink-0 select-none",
        className
      )}
    >
      {getInitials(name)}
    </div>
  );
}
