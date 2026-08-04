// ============================================================
// SuIM SDK — Group members incremental sync (OpenIM IncrSyncGroupMember)
// ============================================================
import * as rest from "../core/rest";
import { getIdb } from "../cache/idb";
import { memoryCache } from "../cache/memory";
import { parseGroupId } from "../core/ids";
import { wsManager } from "../listener/ws";
import type { GroupMemberInfo } from "@/types";

let syncUserId: string | null = null;
const inflight = new Map<string, Promise<GroupMemberInfo[]>>();
const pending = new Set<string>();

export function setGroupMemberSyncUser(userId: string | null): void {
  syncUserId = userId;
  if (!userId) {
    inflight.clear();
    pending.clear();
  }
}

function sortMembers(list: GroupMemberInfo[]): GroupMemberInfo[] {
  return [...list].sort((a, b) => {
    if (b.roleLevel !== a.roleLevel) return b.roleLevel - a.roleLevel;
    return (a.joinedAt || "").localeCompare(b.joinedAt || "");
  });
}

function mergeMember(info: GroupMemberInfo, prev?: GroupMemberInfo): GroupMemberInfo {
  return {
    userId: info.userId,
    groupId: info.groupId || prev?.groupId || "",
    displayName: info.displayName || prev?.displayName || info.username || info.userId,
    username: info.username || prev?.username || "",
    avatar: info.avatar || prev?.avatar || "",
    roleLevel: info.roleLevel ?? prev?.roleLevel ?? 0,
    muteEndTime: info.muteEndTime ?? prev?.muteEndTime ?? 0,
    joinedAt: info.joinedAt || prev?.joinedAt || "",
  };
}

async function applyFull(
  userId: string,
  groupId: string,
  version: number,
  versionId: string
): Promise<GroupMemberInfo[]> {
  const members = await rest.getAllGroupMembers(groupId);
  const idb = getIdb(userId);
  await idb.replaceGroupMembers(groupId, members);
  const full = await rest.getFullGroupMemberUserIDs(groupId).catch(() => null);
  const ver = full?.version ?? version;
  const verId = full?.versionId || versionId;
  await idb.putGroupMemberSyncVersion(groupId, ver, verId);
  const sorted = sortMembers(members);
  memoryCache.setGroupMembers(groupId, sorted);
  wsManager.emit("group.member.synced", { groupId, full: true, members: sorted });
  return sorted;
}

async function applyIncremental(
  userId: string,
  groupId: string,
  res: Awaited<ReturnType<typeof rest.getIncrementalGroupMember>>
): Promise<GroupMemberInfo[]> {
  const idb = getIdb(userId);
  const local =
    memoryCache.getGroupMembers(groupId).length > 0
      ? memoryCache.getGroupMembers(groupId)
      : await idb.getGroupMembers(groupId);
  const map = new Map(local.map((m) => [m.userId, m]));

  const deleted: string[] = [];
  const upserted: GroupMemberInfo[] = [];

  for (const id of res.delete) {
    if (map.delete(id)) deleted.push(id);
  }
  for (const info of [...res.insert, ...res.update]) {
    const m = mergeMember({ ...info, groupId }, map.get(info.userId));
    map.set(m.userId, m);
    upserted.push(m);
  }

  if (deleted.length) await idb.deleteGroupMembers(groupId, deleted);
  if (upserted.length) await idb.putGroupMembers(upserted);

  if (res.sortVersion > 0) {
    const fullIds = await rest.getFullGroupMemberUserIDs(groupId);
    const ordered: GroupMemberInfo[] = [];
    for (const uid of fullIds.userIds) {
      const m = map.get(uid);
      if (m) ordered.push(m);
    }
    // Keep any members not in full list (shouldn't happen) at end.
    for (const m of map.values()) {
      if (!fullIds.userIds.includes(m.userId)) ordered.push(m);
    }
    await idb.replaceGroupMembers(groupId, ordered);
    await idb.putGroupMemberSyncVersion(groupId, res.version, res.versionId);
    memoryCache.setGroupMembers(groupId, ordered);
    if (res.group) memoryCache.putGroup(res.group);
    wsManager.emit("group.member.synced", { groupId, full: false, members: ordered });
    return ordered;
  }

  await idb.putGroupMemberSyncVersion(groupId, res.version, res.versionId);
  const sorted = sortMembers([...map.values()]);
  memoryCache.setGroupMembers(groupId, sorted);
  if (res.group) memoryCache.putGroup(res.group);
  if (deleted.length || upserted.length || res.group) {
    wsManager.emit("group.member.synced", { groupId, full: false, members: sorted });
  }
  return sorted;
}

async function runIncrSyncGroupMembers(
  userId: string,
  gid: string
): Promise<GroupMemberInfo[]> {
  try {
    do {
      pending.delete(gid);
      if (syncUserId !== userId) break;

      if (!memoryCache.getGroupMembers(gid).length) {
        const local = await getIdb(userId).getGroupMembers(gid);
        if (local.length) memoryCache.setGroupMembers(gid, sortMembers(local));
      }

      const ver = await getIdb(userId).getGroupMemberSyncVersion(gid);
      if (syncUserId !== userId) break;
      const res = await rest.getIncrementalGroupMember(gid, ver.versionId, ver.version);
      if (syncUserId !== userId) break;

      if (res.full) {
        await applyFull(userId, gid, res.version, res.versionId);
      } else if (
        res.delete.length ||
        res.insert.length ||
        res.update.length ||
        res.sortVersion > 0 ||
        res.group ||
        res.version !== ver.version ||
        res.versionId !== ver.versionId
      ) {
        await applyIncremental(userId, gid, res);
      }
    } while (pending.has(gid) && syncUserId === userId);
  } catch (err) {
    if (process.env.NODE_ENV === "development") {
      console.warn("[member_sync] incrSyncGroupMembers failed", gid, err);
    }
  }
  return memoryCache.getGroupMembers(gid);
}

/** OpenIM: IncrSyncGroupMember — sync one group's members by version watermark. */
export async function incrSyncGroupMembers(groupId: string): Promise<GroupMemberInfo[]> {
  const userId = syncUserId;
  const gid = parseGroupId(groupId);
  if (!userId || !gid) return memoryCache.getGroupMembers(gid);

  const existing = inflight.get(gid);
  if (existing) {
    pending.add(gid);
    return existing;
  }

  const p = runIncrSyncGroupMembers(userId, gid).finally(() => {
    inflight.delete(gid);
  });
  inflight.set(gid, p);
  return p;
}

/** After join-group sync: refresh members for currently joined groups (login path). */
export async function incrSyncJoinedGroupMembers(groupIds?: string[]): Promise<void> {
  const ids =
    groupIds?.map(parseGroupId).filter(Boolean) ??
    memoryCache.groups.map((g) => g.groupId).filter(Boolean);
  for (const id of ids) {
    await incrSyncGroupMembers(id);
  }
}
