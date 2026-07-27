"use client";

// ============================================================
// EmptyChat — 未选择会话时的占位
// ============================================================
import React from "react";
import { MessageCircle } from "lucide-react";

export default function EmptyChat() {
  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-gray-50 text-gray-400">
      <div className="w-20 h-20 rounded-full bg-indigo-50 flex items-center justify-center mb-4">
        <MessageCircle className="w-10 h-10 text-indigo-300" />
      </div>
      <h3 className="text-lg font-medium text-gray-500 mb-1">欢迎使用 SuIM</h3>
      <p className="text-sm">选择一个会话开始聊天</p>
      <p className="text-xs mt-2 text-gray-300">或使用 Ctrl+N 创建新对话</p>
    </div>
  );
}
