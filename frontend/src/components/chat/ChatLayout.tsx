"use client";

// ============================================================
// ChatLayout — 聊天页面主布局（可拖拽调整面板宽度）
// ============================================================
import React, { useState, useRef, useCallback, useEffect } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useChat } from "@/contexts/ChatContext";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import ConversationList from "./ConversationList";
import ChatArea from "./ChatArea";
import EmptyChat from "./EmptyChat";
import { getConversationById, getMessagesByConversationId } from "@/data/mock";

const MIN_PANEL_W = 140;
const MAX_PANEL_W = 600;
const DEFAULT_PANEL_W = 320;

export default function ChatLayout() {
  const { user, isLoading } = useAuth();
  const { activeConversationId, messages } = useChat();
  const router = useRouter();
  const [panelWidth, setPanelWidth] = useState(DEFAULT_PANEL_W);
  const [dragging, setDragging] = useState(false);
  const dragStartX = useRef(0);
  const dragStartW = useRef(DEFAULT_PANEL_W);

  const handleDragStart = useCallback((e: React.MouseEvent) => {
    setDragging(true);
    dragStartX.current = e.clientX;
    dragStartW.current = panelWidth;
    e.preventDefault();
  }, [panelWidth]);

  useEffect(() => {
    if (!dragging) return;
    const handleMove = (e: MouseEvent) => {
      const delta = e.clientX - dragStartX.current;
      const newW = Math.min(MAX_PANEL_W, Math.max(MIN_PANEL_W, dragStartW.current + delta));
      setPanelWidth(newW);
    };
    const handleUp = () => setDragging(false);
    window.addEventListener("mousemove", handleMove);
    window.addEventListener("mouseup", handleUp);
    return () => {
      window.removeEventListener("mousemove", handleMove);
      window.removeEventListener("mouseup", handleUp);
    };
  }, [dragging]);

  React.useEffect(() => {
    if (!isLoading && !user && process.env.NODE_ENV !== "development") {
      router.replace("/login");
    }
  }, [user, isLoading, router]);

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full" />
      </div>
    );
  }

  const activeConversation = activeConversationId
    ? getConversationById(activeConversationId)
    : null;
  const activeMessages = activeConversationId
    ? messages[activeConversationId] || getMessagesByConversationId(activeConversationId)
    : [];

  return (
    <div className={cn("h-screen flex bg-white overflow-hidden", dragging && "select-none cursor-col-resize")}>
      {/* ConversationList 自带垂直导航栏 + 内容面板 */}
      <div style={{ width: panelWidth }} className="flex-shrink-0 border-r border-gray-100">
        <ConversationList panelWidth={panelWidth} />
      </div>

      {/* 拖拽手柄 */}
      <div
        onMouseDown={handleDragStart}
        className={cn(
          "w-1.5 flex-shrink-0 cursor-col-resize relative z-30",
          "hover:bg-indigo-400/30 active:bg-indigo-400/50 transition-colors",
          dragging && "bg-indigo-400/50"
        )}
      >
        <div className="absolute inset-y-0 -left-1 -right-1" />
      </div>

      {/* 右侧聊天区 */}
      <div className="flex-1 flex flex-col min-w-0">
        {activeConversation ? (
          <ChatArea conversation={activeConversation} messages={activeMessages} />
        ) : (
          <div className="hidden md:flex flex-1"><EmptyChat /></div>
        )}
      </div>
    </div>
  );
}
