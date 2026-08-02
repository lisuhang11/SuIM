// ============================================================
// SuIM SDK — Joined groups incremental sync (OpenIM IncrSyncJoinGroup)
// ============================================================
import * as rest from "../core/rest";
import { getIdb } from "../cache/idb";
import { memoryCache } from "../cache/memory";
import { wsManager } from "../listener/ws";
import type { Group } from "@/types";

let syncUserId: string | null = null;
let inflight: Promise<Group[]> | null = null;
let pending = false;

export function setJoinedGroupSyncUser(userId: string | null): void {
  syncUserId = userId;
}

function sortGroups(list: Group[]): Group[] {
  return [...list].sort((a, b) =>
    (a.name || a.groupId).localeCompare(b.name || b.groupId, "zh")
  );
}

function mergeGroup(info: Group, prev?: Group): Group {
  return {
    groupId: info.groupId,
    name: info.name || prev?.name || info.groupId,
    avatar: info.avatar || prev?.avatar || "",
    ownerId: info.ownerId || prev?.ownerId || "",
    memberCount: info.memberCount || prev?.memberCount || 0,
    introduction: info.introduction ?? prev?.introduction,
    notification: info.notification ?? prev?.notification,
    needVerification: info.needVerification ?? prev?.needVerification,
    isMuted: info.isMuted ?? prev?.isMuted,
    createdAt: info.createdAt || prev?.createdAt || "",
  };
}

async function applyFull(userId: string, version: number, versionId: string): Promise<void> {
  const groups = await rest.getAllJoinedGroups();
  const idb = getIdb(userId);
  await idb.replaceAllJoinedGroups(groups);
  await rest.getFullJoinGroupIDs().catch(() => [] as string[]);
  await idb.putJoinedGroupSyncVersion(version, versionId);
  memoryCache.groups = sortGroups(groups);
  wsManager.emit("group.synced", { full: true, groups: memoryCache.groups });
}

async function applyIncremental(
  userId: string,
  res: Awaited<ReturnType<typeof rest.getIncrementalJoinGroup>>
): Promise<void> {
  const idb = getIdb(userId);
  const map = new Map(
    (memoryCache.groups.length ? memoryCache.groups : await idb.getAllJoinedGroups()).map((g) => [
      g.groupId,
      g,
    ])
  );

  const deleted: string[] = [];
  const upserted: Group[] = [];

  for (const id of res.delete) {
    if (map.delete(id)) deleted.push(id);
  }
  for (const info of [...res.insert, ...res.update]) {
    const g = mergeGroup(info, map.get(info.groupId));
    map.set(g.groupId, g);
    upserted.push(g);
  }

  if (deleted.length) await idb.deleteJoinedGroups(deleted);
  if (upserted.length) await idb.putJoinedGroups(upserted);
  await idb.putJoinedGroupSyncVersion(res.version, res.versionId);

  memoryCache.groups = sortGroups([...map.values()]);
  if (deleted.length || upserted.length || res.sortVersion > 0) {
    wsManager.emit("group.synced", { full: false, groups: memoryCache.groups });
  }
}

async function runIncrSyncJoinedGroups(userId: string): Promise<Group[]> {
  try {
    do {
      pending = false;
      if (syncUserId !== userId) break;

      if (!memoryCache.groups.length) {
        const local = await getIdb(userId).getAllJoinedGroups();
        if (local.length) memoryCache.groups = sortGroups(local);
      }

      const ver = await getIdb(userId).getJoinedGroupSyncVersion();
      if (syncUserId !== userId) break;
      const res = await rest.getIncrementalJoinGroup(ver.versionId, ver.version);
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
      }
    } while (pending && syncUserId === userId);
  } catch (err) {
    if (process.env.NODE_ENV === "development") {
      console.warn("[group_sync] incrSyncJoinedGroups failed", err);
    }
  }
  return memoryCache.groups;
}

/** OpenIM: IncrSyncJoinGroup */
export async function incrSyncJoinedGroups(): Promise<Group[]> {
  const userId = syncUserId;
  if (!userId) return memoryCache.groups;

  if (inflight) {
    pending = true;
    return inflight;
  }

  inflight = runIncrSyncJoinedGroups(userId).finally(() => {
    inflight = null;
  });
  return inflight;
}
