"use client";

// ============================================================
// LoginForm — 登录表单
// ============================================================
import React, { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Eye, EyeOff, MessageCircle } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { cn } from "@/lib/utils";

export default function LoginForm() {
  const router = useRouter();
  const { login, isLoading, error, clearError } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPwd, setShowPwd] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await login({ username, password });
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
          <h1 className="text-2xl font-bold text-gray-900">登录 SuIM</h1>
          <p className="text-gray-500 mt-1">使用你的账号登录即时通讯</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="bg-white rounded-2xl shadow-xl shadow-gray-200/50 p-8 space-y-5">
          {error && (
            <div className="bg-red-50 text-red-600 px-4 py-3 rounded-xl text-sm">
              {error}
              <button onClick={clearError} className="float-right font-bold">&times;</button>
            </div>
          )}

          {/* Username */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">用户名 / 邮箱</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="请输入用户名或邮箱"
              className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm"
              required
              autoComplete="username"
            />
          </div>

          {/* Password */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1.5">密码</label>
            <div className="relative">
              <input
                type={showPwd ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="请输入密码"
                className="w-full px-4 py-3 rounded-xl border border-gray-200 focus:border-indigo-400
                  focus:ring-2 focus:ring-indigo-100 outline-none transition-all text-sm pr-10"
                required
                autoComplete="current-password"
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

          {/* Submit */}
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
            {isLoading ? "登录中..." : "登录"}
          </button>

          {/* Register Link */}
          <p className="text-center text-sm text-gray-500">
            还没有账号？{" "}
            <Link href="/register" className="text-indigo-500 hover:text-indigo-600 font-medium">
              立即注册
            </Link>
          </p>
        </form>


      </div>
    </div>
  );
}
