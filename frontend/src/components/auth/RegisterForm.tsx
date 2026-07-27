"use client";

// ============================================================
// RegisterForm — 注册表单
// ============================================================
import React, { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Eye, EyeOff, MessageCircle } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";

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
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-indigo-50 via-white to-purple-50 p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-indigo-500 text-white mb-4 shadow-lg shadow-indigo-200">
            <MessageCircle className="w-8 h-8" />
          </div>
          <h1 className="text-2xl font-bold text-gray-900">注册 SuIM</h1>
          <p className="text-gray-500 mt-1">创建你的即时通讯账号</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="bg-white rounded-2xl shadow-xl shadow-gray-200/50 p-8 space-y-4">
          {error && (
            <div className="bg-red-50 text-red-600 px-4 py-3 rounded-xl text-sm">
              {error}
              <button onClick={clearError} className="float-right font-bold">&times;</button>
            </div>
          )}
          {localErr && (
            <div className="bg-red-50 text-red-600 px-4 py-3 rounded-xl text-sm">{localErr}</div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">用户名</label>
            <input
              type="text"
              value={form.username}
              onChange={(e) => update("username", e.target.value)}
              placeholder="用于登录的唯一用户名"
              className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm"
              required
              minLength={3}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">显示名称</label>
            <input
              type="text"
              value={form.displayName}
              onChange={(e) => update("displayName", e.target.value)}
              placeholder="别人看到的名字"
              className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">邮箱</label>
            <input
              type="email"
              value={form.email}
              onChange={(e) => update("email", e.target.value)}
              placeholder="your@email.com"
              className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                value={form.password}
                onChange={(e) => update("password", e.target.value)}
                placeholder="至少 6 位密码"
                className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                  focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm pr-10"
                required
                minLength={6}
              />
              <button
                type="button"
                onClick={() => setShowPwd(!showPwd)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
              >
                {showPwd ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">确认密码</label>
            <input
              type="password"
              value={form.confirmPassword}
              onChange={(e) => update("confirmPassword", e.target.value)}
              placeholder="再次输入密码"
              className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm"
              required
              minLength={6}
            />
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className={cn(
              "w-full py-3 rounded-xl font-semibold text-white transition-all",
              "bg-indigo-500 hover:bg-indigo-600 active:scale-[0.98]",
              "shadow-lg shadow-indigo-200",
              isLoading && "opacity-70 cursor-not-allowed"
            )}
          >
            {isLoading ? "注册中..." : "注册"}
          </button>

          <p className="text-center text-sm text-gray-500">
            已有账号？{" "}
            <Link href="/login" className="text-indigo-500 hover:text-indigo-600 font-medium">
              立即登录
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
