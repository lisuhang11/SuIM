// ============================================================
// 根布局 — Providers 包装 + Signal Zinc 字体
// ============================================================
import type { Metadata } from "next";
import { Outfit, Noto_Sans_SC } from "next/font/google";
import { AuthProvider } from "@/contexts/AuthContext";
import { ChatProvider } from "@/contexts/ChatContext";
import { ThemeProvider } from "@/contexts/ThemeContext";
import "./globals.css";

const outfit = Outfit({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

const notoSansSC = Noto_Sans_SC({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-cn",
  display: "swap",
});

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
    <html
      lang="zh-CN"
      className={`${outfit.variable} ${notoSansSC.variable}`}
      suppressHydrationWarning
    >
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var m=localStorage.getItem('suim-theme');var d=m==='dark'||(m!=='light'&&window.matchMedia('(prefers-color-scheme: dark)').matches);var r=document.documentElement;r.classList.toggle('dark',d);r.style.colorScheme=d?'dark':'light';}catch(e){}})();`,
          }}
        />
      </head>
      <body className="font-sans antialiased bg-surface text-ink">
        <ThemeProvider>
          <AuthProvider>
            <ChatProvider>{children}</ChatProvider>
          </AuthProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
