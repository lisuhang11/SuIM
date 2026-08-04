// ============================================================
// SuIM SDK — Friend incremental sync (OpenIM-style IncrSyncFriends)
// ============================================================
import * as rest from "../core/rest";
import type { IncrementalFriendInfo } from "../core/rest";
import { getIdb } from "../cache/idb";
import { memoryCache } from "../cache/memory";
import { wsManager } from "../listener/ws";
import type { Contact } from "@/types";

let syncUserId: string | null = null;
/** In-flight sync promise — concurrent callers must await this, not return empty. */
let inflight: Promise<Contact[]> | null = null;
let pending = false;

export function setFriendSyncUser(userId: string | null): void {
  syncUserId = userId;
}

function infoToContact(info: IncrementalFriendInfo, prev?: Contact): Contact {
  const nickname = info.nickname || prev?.nickname || info.friendUserId;
  const remark = info.remark || undefined;
  return {
    userId: info.friendUserId,
    displayName: remark || nickname,
    nickname,
    username: prev?.username || info.friendUserId,
    avatar: info.avatarUrl || prev?.avatar || "",
    status: prev?.status ?? "offline",
    isFriend: true,
    remark,
    isPinned: info.isPinned,
    lastSeen: prev?.lastSeen,
  };
}

function sortFriends(list: Contact[]): Contact[] {
  return [...list].sort((a, b) => {
    const pin = Number(Boolean(b.isPinned)) - Number(Boolean(a.isPinned));
    if (pin !== 0) return pin;
    return (a.displayName || a.userId).localeCompare(b.displayName || b.userId, "zh");
  });
}

async function applyFull(userId: string, version: number, versionId: string): Promise<void> {
  const friends = await rest.getAllContacts();
  const idb = getIdb(userId);
  await idb.replaceAllFriends(friends);
  // Full IDs 用于与服务端顺序对齐；当前 UI 用 isPinned + 名称排序即可
  await rest.getFullFriendUserIDs().catch(() => [] as string[]);
  await idb.putFriendSyncVersion(version, versionId);
  memoryCache.friends = sortFriends(friends);
  wsManager.emit("friend.synced", { full: true, friends: memoryCache.friends });
}

async function applyIncremental(
  userId: string,
  res: Awaited<ReturnType<typeof rest.getIncrementalFriends>>
): Promise<void> {
  const idb = getIdb(userId);
  const map = new Map((memoryCache.friends.length ? memoryCache.friends : await idb.getAllFriends()).map((c) => [c.userId, c]));

  const deleted: string[] = [];
  const added: Contact[] = [];
  const updated: Contact[] = [];

  for (const id of res.delete) {
    if (map.delete(id)) deleted.push(id);
  }
  for (const info of res.insert) {
    const c = infoToContact(info, map.get(info.friendUserId));
    map.set(c.userId, c);
    added.push(c);
  }
  for (const info of res.update) {
    const c = infoToContact(info, map.get(info.friendUserId));
    map.set(c.userId, c);
    updated.push(c);
  }

  if (deleted.length) await idb.deleteFriends(deleted);
  if (added.length || updated.length) await idb.putFriends([...added, ...updated]);
  await idb.putFriendSyncVersion(res.version, res.versionId);

  memoryCache.friends = sortFriends([...map.values()]);

  for (const c of added) wsManager.emit("friend.added", { friend: c });
  for (const id of deleted) wsManager.emit("friend.deleted", { userId: id });
  for (const c of updated) wsManager.emit("friend.updated", { friend: c });
  if (added.length || deleted.length || updated.length || res.sortVersion > 0) {
    wsManager.emit("friend.synced", { full: false, friends: memoryCache.friends });
  }
}

async function runIncrSyncFriends(userId: string): Promise<Contact[]> {
  try {
    do {
      pending = false;
      if (syncUserId !== userId) break;

      // 冷启动：先灌内存缓存
      if (!memoryCache.friends.length) {
        const local = await getIdb(userId).getAllFriends();
        if (local.length) memoryCache.friends = sortFriends(local);
      }

      const ver = await getIdb(userId).getFriendSyncVersion();
      if (syncUserId !== userId) break;
      const res = await rest.getIncrementalFriends(ver.versionId, ver.version);
      if (syncUserId !== userId) break;

      if (res.full) {
        await applyFull(userId, res.version, res.versionId);
      } else if (
        res.delete.length ||
        res.insert.length ||
        res.update.length ||
        res.version !== ver.version ||
        res.versionId !== ver.versionId
      ) {
        await applyIncremental(userId, res);
      } else if (res.versionId && res.versionId !== ver.versionId) {
        await getIdb(userId).putFriendSyncVersion(res.version, res.versionId);
      }
    } while (pending && syncUserId === userId);
  } catch (err) {
    if (process.env.NODE_ENV === "development") {
      console.warn("[friend_sync] incrSyncFriends failed", err);
    }
  }
  return memoryCache.friends;
}

/** OpenIM: IncrSyncFriends */
export async function incrSyncFriends(): Promise<Contact[]> {
  const userId = syncUserId;
  if (!userId) return memoryCache.friends;

  if (inflight) {
    pending = true;
    return inflight;
  }

  inflight = runIncrSyncFriends(userId).finally(() => {
    inflight = null;
  });
  return inflight;
}
