// ============================================================
// 根布局 — Providers 包装
// ============================================================
import type { Metadata } from "next";
import { AuthProvider } from "@/contexts/AuthContext";
import { ChatProvider } from "@/contexts/ChatContext";
import "./globals.css";

export const metadata: Metadata = {
  title: "SuIM — 即时通讯",
  description: "SuIM 微服务即时通讯系统 Web 客户端",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
      </head>
      <body className="antialiased bg-gray-50 text-gray-900">
        <AuthProvider>
          <ChatProvider>{children}</ChatProvider>
        </AuthProvider>
      </body>
    </html>
  );
}
