"use client";

// ============================================================
// RegisterForm — Signal Zinc
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

export default function RegisterForm() {
  const router = useRouter();
  const { register, isLoading, error, clearError } = useAuth();
  const [form, setForm] = useState({
    username: "",
    displayName: "",
    email: "",
    password: "",
    confirmPassword: "",
  });
  const [showPwd, setShowPwd] = useState(false);
  const [localErr, setLocalErr] = useState("");

  const update = (key: string, value: string) =>
    setForm((f) => ({ ...f, [key]: value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLocalErr("");

    if (form.password !== form.confirmPassword) {
      setLocalErr("两次输入的密码不一致");
      return;
    }
    if (form.password.length < 6) {
      setLocalErr("密码长度不能少于 6 位");
      return;
    }

    try {
      await register({
        username: form.username,
        displayName: form.displayName,
        email: form.email,
        password: form.password,
      });
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
          <h1 className="text-2xl font-semibold tracking-tight text-ink">注册 SuIM</h1>
          <p className="mt-1 text-sm text-ink-muted">创建你的即时通讯账号</p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="space-y-4 rounded-control border border-edge bg-surface-elevated p-8 shadow-panel"
        >
          {error && (
            <div className="rounded-control bg-danger-soft px-4 py-3 text-sm text-danger">
              {error}
              <button type="button" onClick={clearError} className="float-right font-bold">
                &times;
              </button>
            </div>
          )}
          {localErr && (
            <div className="rounded-control bg-danger-soft px-4 py-3 text-sm text-danger">{localErr}</div>
          )}

          {(
            [
              ["username", "用户名", "用于登录的唯一用户名", "text"],
              ["displayName", "显示名称", "别人看到的名字", "text"],
              ["email", "邮箱", "your@email.com", "email"],
            ] as const
          ).map(([key, label, placeholder, type]) => (
            <div key={key} className="space-y-2">
              <label className="block text-sm font-medium text-ink">{label}</label>
              <input
                type={type}
                value={form[key]}
                onChange={(e) => update(key, e.target.value)}
                placeholder={placeholder}
                className={fieldClass}
                required
                minLength={key === "username" ? 3 : undefined}
              />
            </div>
          ))}

          <div className="space-y-2">
            <label className="block text-sm font-medium text-ink">密码</label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                value={form.password}
                onChange={(e) => update("password", e.target.value)}
                placeholder="至少 6 位密码"
                className={cn(fieldClass, "pr-10")}
                required
                minLength={6}
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

          <div className="space-y-2">
            <label className="block text-sm font-medium text-ink">确认密码</label>
            <input
              type="password"
              value={form.confirmPassword}
              onChange={(e) => update("confirmPassword", e.target.value)}
              placeholder="再次输入密码"
              className={fieldClass}
              required
              minLength={6}
            />
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className={cn(
              "ui-press w-full rounded-control bg-accent py-3 text-sm font-semibold text-accent-fg hover:bg-accent-hover",
              isLoading && "cursor-not-allowed opacity-70"
            )}
          >
            {isLoading ? "注册中..." : "注册"}
          </button>

          <p className="text-center text-sm text-ink-muted">
            已有账号？{" "}
            <Link href="/login" className="font-medium text-accent hover:text-accent-hover">
              立即登录
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
