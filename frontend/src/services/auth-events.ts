// ============================================================
// 认证失效事件 — API 401 / WS kick 与 AuthContext 解耦同步
// ============================================================

type AuthExpiredListener = (reason: "unauthorized" | "kick") => void;

const listeners = new Set<AuthExpiredListener>();

export function onAuthExpired(listener: AuthExpiredListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function emitAuthExpired(reason: "unauthorized" | "kick" = "unauthorized"): void {
  listeners.forEach((fn) => {
    try {
      fn(reason);
    } catch {
      // ignore listener errors
    }
  });
}
