"use client";

// ============================================================
// MessageInput — 消息输入框 + 发送按钮
// ============================================================
import React, { useState, useRef, useCallback, useEffect } from "react";
import { Send, Paperclip, Smile } from "lucide-react";
import { cn } from "@/lib/utils";

interface MessageInputProps {
  onSend: (content: string) => void;
  onTyping: (isTyping: boolean) => void;
  disabled?: boolean;
}

export default function MessageInput({ onSend, onTyping, disabled }: MessageInputProps) {
  const [text, setText] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  // 发送消息
  const handleSend = useCallback(() => {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setText("");
    onTyping(false);

    // 重置输入框高度
    if (inputRef.current) {
      inputRef.current.style.height = "auto";
    }
  }, [text, onSend, onTyping]);

  // 键盘事件
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // 输入变化
  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setText(e.target.value);

    // 自动调整高度
    const el = e.target;
    el.style.height = "auto";
    el.style.height = Math.min(el.scrollHeight, 120) + "px";

    // 正在输入
    const isTyping = e.target.value.length > 0;
    onTyping(isTyping);

    // 防抖：停止输入后发送
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    if (isTyping) {
      typingTimerRef.current = setTimeout(() => onTyping(false), 3000);
    }
  };

  useEffect(() => {
    return () => {
      if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    };
  }, []);

  return (
    <div className="border-t border-gray-200 bg-white px-4 py-3">
      <div className="flex items-end gap-2">
        {/* 附件按钮 */}
        <button
          className={cn(
            "p-2 rounded-xl text-gray-400 hover:text-indigo-500 hover:bg-indigo-50 transition-colors",
            disabled && "opacity-50 cursor-not-allowed"
          )}
          disabled={disabled}
          title="发送文件"
        >
          <Paperclip className="w-5 h-5" />
        </button>

        {/* 输入框 */}
        <div className="flex-1 relative">
          <textarea
            ref={inputRef}
            value={text}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            placeholder="输入消息..."
            rows={1}
            disabled={disabled}
            className={cn(
              "w-full resize-none rounded-xl border border-gray-200 px-4 py-2.5 text-sm",
              "focus:outline-none focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100",
              "placeholder:text-gray-400 max-h-[120px] transition-all",
              disabled && "bg-gray-50 cursor-not-allowed"
            )}
          />
          {/* Emoji 按钮 */}
          <button
            className={cn(
              "absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-lg",
              "text-gray-400 hover:text-amber-500 hover:bg-amber-50 transition-colors",
              disabled && "opacity-50 cursor-not-allowed"
            )}
            disabled={disabled}
            title="表情"
          >
            <Smile className="w-5 h-5" />
          </button>
        </div>

        {/* 发送按钮 */}
        <button
          onClick={handleSend}
          disabled={disabled || !text.trim()}
          className={cn(
            "p-2.5 rounded-xl transition-all flex-shrink-0",
            "bg-indigo-500 text-white shadow-md shadow-indigo-200",
            "hover:bg-indigo-600 active:scale-95",
            (disabled || !text.trim()) && "bg-gray-300 shadow-none cursor-not-allowed hover:bg-gray-300"
          )}
        >
          <Send className="w-5 h-5" />
        </button>
      </div>
    </div>
  );
}
