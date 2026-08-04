"use client";

// AuthContext ? session / auth state (IMSDK)
import React, { createContext, useContext, useState, useCallback, useEffect, useRef } from "react";
import type { User, LoginRequest, RegisterRequest } from "@/types";
import { IMSDK } from "@/suim-sdk";
import * as storage from "@/services/storage";
import { onAuthExpired } from "@/services/auth-events";
import { isMockMode, mockCurrentUser } from "@/services/mock-data";

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
  changePassword: (oldPassword: string, newPassword: string) => Promise<void>;
  updateProfile: (data: { nickname: string; avatarFile?: File }) => Promise<User>;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function clearSessionLocally() {
  IMSDK.disconnect();
  IMSDK.markLoggedIn(false);
  storage.clearAll();
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isLoading: true,
    isAuthenticated: false,
    error: null,
  });
  const loggingOutRef = useRef(false);

  const forceLogout = useCallback((message?: string) => {
    if (loggingOutRef.current) return;
    loggingOutRef.current = true;
    clearSessionLocally();
    setState({
      user: null,
      isLoading: false,
      isAuthenticated: false,
      error: message || null,
    });
    loggingOutRef.current = false;
  }, []);

  useEffect(() => {
    let cancelled = false;
    const init = async () => {
      if (isMockMode) {
        if (!cancelled) {
          setState({
            user: mockCurrentUser,
            isLoading: false,
            isAuthenticated: true,
            error: null,
          });
        }
        return;
      }

      const token = storage.getToken();
      if (!token) {
        storage.removeCachedUser();
        if (!cancelled) setState((s) => ({ ...s, isLoading: false }));
        return;
      }

      try {
        const user = await IMSDK.getSelfUserInfo();
        if (cancelled) return;
        storage.setCachedUser(user);
        IMSDK.markLoggedIn(true);
        setState({
          user,
          isLoading: false,
          isAuthenticated: true,
          error: null,
        });
        IMSDK.connect();
      } catch {
        if (cancelled) return;
        clearSessionLocally();
        setState({
          user: null,
          isLoading: false,
          isAuthenticated: false,
          error: null,
        });
      }
    };

    void init();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    const unsubAuth = onAuthExpired((reason) => {
      forceLogout(
        reason === "kick"
          ? "Account logged in elsewhere or session expired"
          : "Session expired, please login again"
      );
    });
    const unsubKick = IMSDK.on("kick", () => {
      forceLogout("Account logged in elsewhere or session expired");
    });
    return () => {
      unsubAuth();
      unsubKick();
    };
  }, [forceLogout]);

  const login = useCallback(async (data: LoginRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    if (isMockMode) {
      setState({ user: mockCurrentUser, isLoading: false, isAuthenticated: true, error: null });
      return;
    }
    try {
      const res = await IMSDK.login(data);
      if (!res.token || !res.user?.userId) {
        throw new Error("Invalid login response");
      }
      storage.setToken(res.token);
      storage.setCachedUser(res.user);
      setState({
        user: res.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Login failed";
      setState((s) => ({ ...s, isLoading: false, error: message }));
      throw err;
    }
  }, []);

  const register = useCallback(async (data: RegisterRequest) => {
    setState((s) => ({ ...s, isLoading: true, error: null }));
    if (isMockMode) {
      setState({
        user: {
          ...mockCurrentUser,
          username: data.username,
          displayName: data.displayName || data.username,
          email: data.email,
        },
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
      return;
    }
    try {
      const res = await IMSDK.register(data);
      if (!res.token) {
        const loginRes = await IMSDK.login({
          username: data.email,
          password: data.password,
        });
        if (!loginRes.token || !loginRes.user?.userId) {
          throw new Error("Registered but login failed, please login manually");
        }
        storage.setToken(loginRes.token);
        storage.setCachedUser(loginRes.user);
        setState({
          user: loginRes.user,
          isLoading: false,
          isAuthenticated: true,
          error: null,
        });
        return;
      }
      storage.setToken(res.token);
      storage.setCachedUser(res.user);
      IMSDK.markLoggedIn(true);
      IMSDK.connect();
      setState({
        user: res.user,
        isLoading: false,
        isAuthenticated: true,
        error: null,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Register failed";
      setState((s) => ({ ...s, isLoading: false, error: message }));
      throw err;
    }
  }, []);

  const logout = useCallback(async () => {
    if (isMockMode) {
      setState({ user: null, isLoading: false, isAuthenticated: false, error: null });
      return;
    }
    try {
      await IMSDK.logout();
    } catch {
      // ignore
    }
    clearSessionLocally();
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

  const changePassword = useCallback(async (oldPassword: string, newPassword: string) => {
    if (isMockMode) return;
    await IMSDK.changePassword(oldPassword, newPassword);
    clearSessionLocally();
    setState({ user: null, isLoading: false, isAuthenticated: false, error: null });
  }, []);

  const updateProfile = useCallback(
    async (data: { nickname: string; avatarFile?: File }) => {
      if (!state.user) throw new Error("Not logged in");
      if (isMockMode) {
        const next = {
          ...state.user,
          displayName: data.nickname,
          username: data.nickname,
          avatar: data.avatarFile ? URL.createObjectURL(data.avatarFile) : state.user.avatar,
        };
        setState((s) => ({ ...s, user: next }));
        return next;
      }
      let avatarUrl: string | undefined;
      if (data.avatarFile) {
        avatarUrl = await IMSDK.uploadAvatar(data.avatarFile, {
          type: "user",
          id: state.user.userId,
        });
      }
      const next = await IMSDK.setSelfInfo({
        nickname: data.nickname,
        ...(avatarUrl ? { avatarUrl } : {}),
      });
      // Ensure avatar is present even if GET /me lags
      const merged = avatarUrl && !next.avatar ? { ...next, avatar: avatarUrl } : next;
      storage.setCachedUser(merged);
      setState((s) => ({ ...s, user: merged }));
      return merged;
    },
    [state.user]
  );

  return (
    <AuthContext.Provider
      value={{ ...state, login, register, logout, changePassword, updateProfile, clearError }}
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
