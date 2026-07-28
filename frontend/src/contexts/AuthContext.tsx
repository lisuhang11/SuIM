"use client";

// ============================================================
// AuthContext — 认证状态全局管理
// ============================================================
import React, { createContext, useContext, useState, useCallback, useEffect } from "react";
import type { User, LoginRequest, RegisterRequest } from "@/types";
import * as api from "@/services/api";
import * as storage from "@/services/storage";
import { wsManager } from "@/services/websocket";

interface AuthState {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  error: string | null;
}

interface AuthContextValue extends AuthState {
  login: (data: LoginRequest) => Promise<void>;
  register: (data: RegisterRequest) => Promise<void>;
  logout: () => Promise<void>;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isLoading: true,
    isAuthenticated: false,
    error: null,
  });

  // 启动时检查本地是否有缓存的 token
  useEffect(() => {
    const init = async () => {
      const token = storage.getToken();
      if (!token) {
        // 没有 token，清除可能的脏数据
        storage.removeCachedUser();
        setState((s) => ({ ...s, isLoading: false }));
        return;
      }

      try {
        // 先用 API 验证 token 是否仍然有效
        const user = await api.getCurrentUser();
        storage.setCachedUser(user);
        setState({
          user,
          isLoading: false,
          isAuthenticated: true,
          error: null,
        });
        // token 验证通过后才建立 WebSocket 连接
        wsManager.connect();
      } catch {
        // Token 过期或无效 — 清除并回到未登录状态
        storage.clearAll();
        setState({
          user: null,
          isLoading: false,
          isAuthenticated: false,
          error: null,
        });
      }
    };

    init();
  }, []);

  const login = useCallback(async (data: LoginRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    try {
      // 联调模式：直接调用真实 API（dev 模式下也走真实请求）
      const res = await api.login(data);
      storage.setToken(res.token);
      storage.setCachedUser(res.user);
      setState({
        user: res.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
      wsManager.connect();
    } catch (err) {
      const message = err instanceof Error ? err.message : "登录失败，请检查后端服务";
      setState((s) => ({ ...s, isLoading: false, error: message }));
      throw err;
    }
  }, []);

  const register = useCallback(async (data: RegisterRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    try {
      // 联调模式：直接调用真实 API
      const res = await api.register(data);
      // 注册后自动登录以获取 token（后端注册接口不返回 token）
      if (!res.token) {
        const loginRes = await api.login({
          username: data.email,
          password: data.password,
        });
        if (loginRes.token) {
          storage.setToken(loginRes.token);
          storage.setCachedUser(loginRes.user);
          setState({
            user: loginRes.user,
            isLoading: false,
            isAuthenticated: true,
            error: null,
          });
          wsManager.connect();
          return;
        }
        // auto-login 没拿到 token，强制回登录页
        throw new Error("注册成功但登录失败，请手动登录");
      }
      if (res.token) storage.setToken(res.token);
      storage.setCachedUser(res.user);
      setState({
        user: res.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
      wsManager.connect();
    } catch (err) {
      const message = err instanceof Error ? err.message : "注册失败，请检查后端服务";
      setState((s) => ({ ...s, isLoading: false, error: message }));
      throw err;
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // 登出 API 调用失败也不影响本地清理
    }
    wsManager.disconnect();
    storage.clearAll();
    setState({
      user: null,
      isLoading: false,
      isAuthenticated: false,
      error: null,
    });
  }, []);

  const clearError = useCallback(() => {
    setState((s) => ({ ...s, error: null }));
  }, []);

  return (
    <AuthContext.Provider
      value={{ ...state, login, register, logout, clearError }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
