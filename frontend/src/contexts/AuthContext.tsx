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
        setState((s) => ({ ...s, isLoading: false }));
        return;
      }

      try {
        const cached = storage.getCachedUser<User>();
        if (cached) {
          setState({
            user: cached,
            isLoading: false,
            isAuthenticated: true,
            error: null,
          });
          wsManager.connect();
        } else {
          // Token 存在但无缓存用户，从 API 获取
          const user = await api.getCurrentUser();
          storage.setCachedUser(user);
          setState({
            user,
            isLoading: false,
            isAuthenticated: true,
            error: null,
          });
          wsManager.connect();
        }
      } catch {
        // Token 过期或无效
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
      const message = err instanceof Error ? err.message : "登录失败";
      setState((s) => ({ ...s, isLoading: false, error: message }));
      throw err;
    }
  }, []);

  const register = useCallback(async (data: RegisterRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    try {
      // 1. 注册账号
      await api.register(data);
      // 2. 注册成功但无 token，自动登录获取 token
      const loginRes = await api.login({ username: data.email, password: data.password });
      storage.setToken(loginRes.token);
      storage.setCachedUser(loginRes.user);
      setState({
        user: loginRes.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
      wsManager.connect();
    } catch (err) {
      const message = err instanceof Error ? err.message : "注册失败";
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
