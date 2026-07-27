"use client";

// ============================================================
// AuthContext — 认证状态全局管理
// ============================================================
import React, { createContext, useContext, useState, useCallback, useEffect } from "react";
import type { User, LoginRequest, RegisterRequest } from "@/types";
import * as api from "@/services/api";
import * as storage from "@/services/storage";
import { wsManager } from "@/services/websocket";
import { mockUsers } from "@/data/mock";

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
    } catch {
      // API 不可用时回退到 mock（便于纯前端调试）
      try {
        const found = mockUsers.find(
          (u) => u.username === data.username || u.email === data.username
        );
        if (!found) throw new Error("用户不存在");

        storage.setToken("mock_dev_token_suim");
        storage.setCachedUser(found);
        setState({
          user: found,
          isLoading: false,
          isAuthenticated: true,
          error: null,
        });
        wsManager.connect();
      } catch (mockErr) {
        const message = mockErr instanceof Error ? mockErr.message : "登录失败";
        setState((s) => ({ ...s, isLoading: false, error: message }));
        throw mockErr;
      }
    }
  }, []);

  const register = useCallback(async (data: RegisterRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    try {
      // 联调模式：直接调用真实 API
      const res = await api.register(data);
      if (res.token) storage.setToken(res.token);
      storage.setCachedUser(res.user);
      setState({
        user: res.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
      wsManager.connect();
    } catch {
      // API 不可用时回退到 mock
      try {
        const newUser: User = {
          userId: `u_${Date.now()}`,
          username: data.username,
          displayName: data.displayName,
          email: data.email,
          avatar: "",
          status: "online",
          createdAt: new Date().toISOString(),
        };
        storage.setToken("mock_dev_token_suim");
        storage.setCachedUser(newUser);
        setState({
          user: newUser,
          isLoading: false,
          isAuthenticated: true,
          error: null,
        });
        wsManager.connect();
      } catch (mockErr) {
        const message = mockErr instanceof Error ? mockErr.message : "注册失败";
        setState((s) => ({ ...s, isLoading: false, error: message }));
        throw mockErr;
      }
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
