// ============================================================
// SuIM SDK — Group module
// ============================================================
import * as rest from "../core/rest";
import { memoryCache } from "../cache/memory";
import { parseGroupId } from "../core/ids";
import { incrSyncJoinedGroups } from "./group_sync";
import { incrSyncGroupMembers } from "./member_sync";
import type {
  CreateGroupRequest,
  Group,
  GroupApplication,
  GroupMemberInfo,
  UpdateGroupRequest,
} from "@/types";

export { groupConversationId, parseGroupId } from "../core/ids";

/**
 * OpenIM: CreateGroup
 * 调用 POST /groups；返回 GroupInfo。
 * 会话由服务端在 group.created 时同步创建（SuIM 保留该策略，conversation_id = gid_<groupId>）。
 */
export async function createGroup(data: CreateGroupRequest): Promise<Group> {
  const group = await rest.createGroup(data);
  if (!group.groupId) {
    throw new Error("create group failed: empty group_id");
  }
  memoryCache.putGroup(group);
  await incrSyncJoinedGroups();
  return group;
}

/** OpenIM: GetJoinedGroupList — 走增量同步后返回本地权威列表 */
export async function getJoinedGroupList(): Promise<Group[]> {
  return incrSyncJoinedGroups();
}

/**
 * OpenIM: GetGroupsInfo
 * L1 memoryCache → miss 再 REST（批量或单条）并回填。
 */
export async function getGroupsInfo(groupIds: string[]): Promise<Group[]> {
  const ids = Array.from(
    new Set(groupIds.map((id) => parseGroupId(id)).filter(Boolean))
  );
  if (ids.length === 0) return [];

  const { hit, miss } = memoryCache.getGroups(ids);
  if (miss.length === 0) return orderGroups(ids, hit);

  const fetched = await rest.getGroupsInfo(miss);
  memoryCache.putGroups(fetched);
  return orderGroups(ids, [...hit, ...fetched]);
}

/** OpenIM 风格单条查询（走 L1 + REST） */
export async function getGroupInfo(groupId: string): Promise<Group> {
  const id = parseGroupId(groupId);
  if (!id) throw new Error("group_id is required");
  const cached = memoryCache.getGroup(id);
  if (cached) return cached;
  const group = await rest.getGroupInfo(id);
  memoryCache.putGroup(group);
  return group;
}

function orderGroups(ids: string[], groups: Group[]): Group[] {
  const map = new Map(groups.map((g) => [g.groupId, g]));
  return ids.map((id) => map.get(id)).filter((g): g is Group => Boolean(g));
}

/** OpenIM: SetGroupInfo */
export async function setGroupInfo(data: UpdateGroupRequest): Promise<void> {
  const groupId = parseGroupId(data.groupId);
  await rest.updateGroupInfo({ ...data, groupId });
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(groupId);
}

/** OpenIM: GetGroupMemberList — 走成员增量同步后返回本地权威列表 */
export async function getGroupMemberList(
  groupId: string,
  params?: { offset?: number; limit?: number }
): Promise<GroupMemberInfo[]> {
  const members = await incrSyncGroupMembers(groupId);
  if (params?.offset !== undefined || params?.limit !== undefined) {
    const offset = params.offset ?? 0;
    const limit = params.limit ?? members.length;
    return members.slice(offset, offset + limit);
  }
  return members;
}

export async function inviteUserToGroup(groupId: string, userIds: string[]): Promise<void> {
  await rest.inviteToGroup(groupId, userIds);
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(groupId);
}

export async function kickGroupMember(groupId: string, userId: string): Promise<void> {
  await rest.kickGroupMember(groupId, userId);
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(groupId);
}

export async function quitGroup(groupId: string): Promise<void> {
  const id = parseGroupId(groupId);
  await rest.quitGroup(id);
  await incrSyncJoinedGroups();
}

export async function dismissGroup(groupId: string): Promise<void> {
  const id = parseGroupId(groupId);
  await rest.dismissGroup(id);
  await incrSyncJoinedGroups();
}

export async function transferGroupOwner(groupId: string, newOwnerId: string): Promise<void> {
  await rest.transferGroupOwner(groupId, newOwnerId);
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(groupId);
}

export async function changeGroupMute(groupId: string, isMuted: boolean): Promise<void> {
  await rest.setGroupMute(groupId, isMuted);
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(groupId);
}

export async function changeGroupMemberMute(
  groupId: string,
  userId: string,
  mutedSeconds: number
): Promise<void> {
  const muteEndTime = mutedSeconds > 0 ? Date.now() + mutedSeconds * 1000 : 0;
  await rest.setMemberMute(groupId, userId, muteEndTime);
  await incrSyncGroupMembers(groupId);
}

export async function joinGroup(groupId: string, message?: string): Promise<void> {
  await rest.applyToJoinGroup(groupId, message);
  await incrSyncJoinedGroups();
}

export async function getGroupApplicationListAsRecipient(
  groupId: string
): Promise<GroupApplication[]> {
  return rest.getPendingApplications(groupId);
}

export async function getGroupApplicationListAsApplicant(): Promise<GroupApplication[]> {
  return rest.getMyApplications();
}

export async function acceptGroupApplication(
  application: Pick<GroupApplication, "groupId" | "userId">,
  handleMsg?: string
): Promise<void> {
  await rest.handleApplication(application, true, handleMsg);
  await incrSyncJoinedGroups();
  await incrSyncGroupMembers(application.groupId);
}

export async function refuseGroupApplication(
  application: Pick<GroupApplication, "groupId" | "userId">,
  handleMsg?: string
): Promise<void> {
  await rest.handleApplication(application, false, handleMsg);
}

export async function getGroupApplicationUnhandledCount(groupId: string): Promise<number> {
  return rest.getUnhandledGroupApplicationCount(groupId);
}

export { incrSyncJoinedGroups } from "./group_sync";
export { incrSyncGroupMembers } from "./member_sync";

export {
  getGroups,
  updateGroupInfo,
  getGroupMembers,
  getPendingApplications,
  getMyApplications,
  handleApplication,
  applyToJoinGroup,
  setGroupMute,
  setMemberMute,
  inviteToGroup,
} from "../core/rest";
