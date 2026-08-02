// ============================================================
// SuIM SDK — Relation / Friend module
// ============================================================
import * as rest from "../core/rest";
import { memoryCache } from "../cache/memory";
import { incrSyncFriends } from "./friend_sync";
import type { Contact, FriendRequest, BlacklistEntry } from "@/types";

/** OpenIM: GetFriendList — 走增量同步后返回本地权威列表 */
export async function getFriendList(): Promise<Contact[]> {
  return incrSyncFriends();
}

/** OpenIM: AddFriend */
export async function addFriend(toUserId: string, message = ""): Promise<void> {
  await rest.sendFriendRequest(toUserId, message);
}

/** OpenIM: AcceptFriendApplication */
export async function acceptFriendApplication(
  fromUserId: string,
  handleMsg?: string
): Promise<void> {
  await rest.respondFriendRequest(fromUserId, 1, handleMsg);
  await incrSyncFriends();
}

/** OpenIM: RefuseFriendApplication */
export async function refuseFriendApplication(
  fromUserId: string,
  handleMsg?: string
): Promise<void> {
  await rest.respondFriendRequest(fromUserId, -1, handleMsg);
}

export async function getFriendApplicationListAsRecipient(
  params?: { handleResults?: number[]; offset?: number; limit?: number }
): Promise<FriendRequest[]> {
  return rest.getIncomingRequests(params);
}

export async function getFriendApplicationListAsApplicant(
  params?: { handleResults?: number[]; offset?: number; limit?: number }
): Promise<FriendRequest[]> {
  return rest.getOutgoingRequests(params);
}

export async function getFriendApplicationUnhandledCount(): Promise<number> {
  return rest.getUnhandledRequestCount();
}

export async function deleteFriend(friendId: string): Promise<void> {
  await rest.deleteFriend(friendId);
  await incrSyncFriends();
}

/** OpenIM-style: UpdateFriends (single friend) */
export async function updateFriend(
  friendId: string,
  patch: { remark?: string; isPinned?: boolean }
): Promise<void> {
  await rest.updateFriend(friendId, patch);
  await incrSyncFriends();
}

/** OpenIM: GetBlackList — 网关分页；结果写入 memoryCache.blacks（首屏或 offset=0 时整表替换） */
export async function getBlackList(params?: {
  offset?: number;
  limit?: number;
}): Promise<BlacklistEntry[]> {
  const offset = params?.offset ?? 0;
  const list = await rest.getBlackList(params);
  if (offset === 0) {
    memoryCache.blacks = list;
  } else {
    const seen = new Set(memoryCache.blacks.map((b) => b.userId));
    memoryCache.blacks = [
      ...memoryCache.blacks,
      ...list.filter((b) => !seen.has(b.userId)),
    ];
  }
  return list;
}

/**
 * OpenIM: AddBlack
 * `ex` 对齐 OpenIM 签名；当前后端 BlockUser 未接收 ex，调用侧可传但不会发往网关。
 * 拉黑不移除好友关系（与后端一致）。
 */
export async function addBlack(userId: string, _ex = ""): Promise<void> {
  await rest.blockUser(userId);
  const friend = memoryCache.friends.find((c) => c.userId === userId);
  const cachedUser = memoryCache.users.get(userId);
  const entry: BlacklistEntry = {
    userId,
    displayName: friend?.displayName || cachedUser?.displayName || userId,
    username: friend?.username || cachedUser?.username || userId,
    avatar: friend?.avatar || cachedUser?.avatar || "",
    createTime: Date.now(),
    ex: _ex || undefined,
  };
  memoryCache.blacks = [entry, ...memoryCache.blacks.filter((b) => b.userId !== userId)];
}

/** OpenIM: RemoveBlack */
export async function removeBlack(userId: string): Promise<void> {
  await rest.unblockUser(userId);
  memoryCache.blacks = memoryCache.blacks.filter((b) => b.userId !== userId);
}

export { incrSyncFriends } from "./friend_sync";

// Compat aliases used by existing UI
export {
  getContacts,
  sendFriendRequest,
  respondFriendRequest,
  getIncomingRequests,
  getOutgoingRequests,
  getUnhandledRequestCount,
} from "../core/rest";
