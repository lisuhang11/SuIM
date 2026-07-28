// ============================================================
// 本地存储工具 — Token / 缓存管理
// ============================================================

const TOKEN_KEY = "suim_token";
const USER_KEY = "suim_user";
const ACTIVE_CONV_KEY = "suim_active_conv";

// ---------- Token ----------
export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

// ---------- User ----------
export function getCachedUser<T>(): T | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(USER_KEY);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

export function setCachedUser<T>(user: T): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

export function removeCachedUser(): void {
  localStorage.removeItem(USER_KEY);
}

// ---------- Active Conversation ----------
export function getActiveConversationId(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(ACTIVE_CONV_KEY);
}

export function setActiveConversationId(id: string): void {
  localStorage.setItem(ACTIVE_CONV_KEY, id);
}

// ---------- Clear All ----------
export function clearAll(): void {
  removeToken();
  removeCachedUser();
  localStorage.removeItem(ACTIVE_CONV_KEY);
}
