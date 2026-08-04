"use client";

// ============================================================
// LoginForm — Signal Zinc
// ============================================================
import React, { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Eye, EyeOff } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";
import { ThemeToggle } from "@/components/shared/ThemeToggle";

const fieldClass =
  "w-full rounded-control border border-edge bg-surface px-4 py-3 text-sm text-ink outline-none transition duration-ui placeholder:text-ink-muted focus:border-accent focus:ring-2 focus:ring-accent/20";

export default function LoginForm() {
  const router = useRouter();
  const { login, isLoading, error, clearError } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPwd, setShowPwd] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await login({ username: email, password });
      router.push("/chat");
    } catch {
      // error is set in context
    }
  };

  return (
    <div className="flex min-h-[100dvh] items-center justify-center bg-surface p-4">
      <div className="absolute right-4 top-4">
        <ThemeToggle compact />
      </div>
      <div className="w-full max-w-md">
        <div className="mb-8 text-center">
          <div className="mb-4 inline-flex h-12 w-12 items-center justify-center rounded-control bg-accent text-lg font-bold text-accent-fg">
            S
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-ink">登录 SuIM</h1>
          <p className="mt-1 text-sm text-ink-muted">使用你的账号登录即时通讯</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="space-y-5 rounded-control border border-edge bg-surface-elevated p-8 shadow-panel"
        >
          {error && (
            <div className="rounded-control bg-danger-soft px-4 py-3 text-sm text-danger">
              {error}
              <button type="button" onClick={clearError} className="float-right font-bold">
                &times;
              </button>
            </div>
          )}

          <div className="space-y-2">
            <label className="block text-sm font-medium text-ink">邮箱</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="请输入邮箱地址"
              className={fieldClass}
              required
              autoComplete="email"
            />
          </div>

          <div className="space-y-2">
            <label className="block text-sm font-medium text-ink">密码</label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="请输入密码"
                className={cn(fieldClass, "pr-10")}
                required
                autoComplete="current-password"
              />
              <button
                type="button"
                onClick={() => setShowPwd(!showPwd)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-muted hover:text-ink"
              >
                {showPwd ? <EyeOff className="h-4 w-4" strokeWidth={1.75} /> : <Eye className="h-4 w-4" strokeWidth={1.75} />}
              </button>
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className={cn(
              "ui-press w-full rounded-control bg-accent py-3 text-sm font-semibold text-accent-fg hover:bg-accent-hover",
              isLoading && "cursor-not-allowed opacity-70"
            )}
          >
            {isLoading ? "登录中..." : "登录"}
          </button>

          <p className="text-center text-sm text-ink-muted">
            还没有账号？{" "}
            <Link href="/register" className="font-medium text-accent hover:text-accent-hover">
              立即注册
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
