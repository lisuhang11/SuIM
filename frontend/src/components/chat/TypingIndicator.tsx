"use client";

// ============================================================
// TypingIndicator — 正在输入动画
// ============================================================
import React from "react";

interface TypingIndicatorProps {
  conversationId: string;
}

export default function TypingIndicator({}: TypingIndicatorProps) {
  return (
    <div className="flex items-center gap-2 px-4 mb-4">
      <div className="bg-white border border-gray-100 rounded-2xl rounded-bl-md px-4 py-3 shadow-sm">
        <div className="flex gap-1">
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce [animation-delay:0ms]" />
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce [animation-delay:150ms]" />
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-bounce [animation-delay:300ms]" />
        </div>
      </div>
      <span className="text-xs text-gray-400">对方正在输入...</span>
    </div>
  );
}
