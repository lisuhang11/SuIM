// ============================================================
// Presence — 好友在线状态订阅（对齐 OpenIM subscribeUsersStatus）
// ============================================================
import type { UserStatus } from "@/types";
import { memoryCache } from "../cache/memory";
import { wsManager } from "../listener/ws";
import { getUsersOnlineStatus as fetchOnlineStatus } from "../core/rest";

export type OnlineStatusItem = {
  userId: string;
  status: UserStatus;
  platformIds?: number[];
};

/** WS 上行：订阅好友在线状态，网关回 presence.snapshot */
export function subscribeUsersStatus(userIds: string[]): boolean {
  const ids = [...new Set(userIds.filter(Boolean))];
  if (!ids.length) return false;
  return wsManager.send("presence.subscribe", { user_ids: ids });
}

export function unsubscribeUsersStatus(userIds: string[]): boolean {
  const ids = [...new Set(userIds.filter(Boolean))];
  if (!ids.length) return false;
  return wsManager.send("presence.unsubscribe", { user_ids: ids });
}

export async function getUsersOnlineStatus(userIds: string[]): Promise<OnlineStatusItem[]> {
  return fetchOnlineStatus(userIds);
}

/** 好友同步完成后：订阅 + REST 拉一次快照并 emit user.status */
export async function syncFriendsAccountsPresence(): Promise<void> {
  const ids = memoryCache.friends.map((f) => f.userId).filter(Boolean);
  if (!ids.length) return;

  subscribeUsersStatus(ids);

  try {
    const statuses = await getUsersOnlineStatus(ids);
    for (const s of statuses) {
      if (!s.userId) continue;
      const friend = memoryCache.friends.find((f) => f.userId === s.userId);
      if (friend) friend.status = s.status;
      wsManager.emit("user.status", { userId: s.userId, status: s.status });
    }
  } catch (err) {
    if (process.env.NODE_ENV === "development") {
      console.warn("[presence] getUsersOnlineStatus failed", err);
    }
  }
}
