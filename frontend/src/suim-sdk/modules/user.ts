// ============================================================
// SuIM SDK — User module (OpenIM-aligned names)
// ============================================================
import * as rest from "../core/rest";
import { memoryCache } from "../cache/memory";
import type { AuthResponse, LoginRequest, RegisterRequest, User } from "@/types";

export async function login(data: LoginRequest): Promise<AuthResponse> {
  const res = await rest.login(data);
  if (res.user?.userId) memoryCache.setSelf(res.user);
  return res;
}

export async function register(data: RegisterRequest): Promise<AuthResponse> {
  const res = await rest.register(data);
  if (res.user?.userId) memoryCache.setSelf(res.user);
  return res;
}

export async function logout(): Promise<void> {
  await rest.logout();
  memoryCache.clear();
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  await rest.changePassword(oldPassword, newPassword);
}

/** OpenIM: GetSelfUserInfo — 经 batch 拉取本人资料 */
export async function getSelfUserInfo(): Promise<User> {
  const cached = memoryCache.selfUser;
  if (cached?.userId) {
    const list = await rest.getUsersBatch([cached.userId]);
    const user = list.find((u) => u.userId === cached.userId) ?? list[0];
    if (user) {
      memoryCache.setSelf(user);
      return user;
    }
  }
  const user = await rest.getCurrentUser();
  memoryCache.setSelf(user);
  return user;
}

/** OpenIM: SetSelfInfo */
export async function setSelfInfo(data: {
  nickname?: string;
  avatarUrl?: string;
}): Promise<User> {
  const user = await rest.updateCurrentUser(data);
  memoryCache.setSelf(user);
  return user;
}

/** OpenIM: GetUsersInfo — cache first, then batch fetch misses */
export async function getUsersInfo(userIds: string[]): Promise<User[]> {
  const unique = Array.from(new Set(userIds.filter(Boolean)));
  if (unique.length === 0) return [];
  const { hit, miss } = memoryCache.getUsers(unique);
  if (miss.length === 0) return hit;
  const fetched = await rest.getUsersBatch(miss);
  memoryCache.putUsers(fetched);
  const byId = new Map([...hit, ...fetched].map((u) => [u.userId, u]));
  return unique.map((id) => byId.get(id)).filter((u): u is User => Boolean(u));
}

export async function searchUsers(query: string): Promise<User[]> {
  const list = await rest.searchUsers(query);
  memoryCache.putUsers(list);
  return list;
}

/** OpenIM: SetGlobalRecvMessageOpt — 0 正常 / 1 不接收 / 2 接收不通知 */
export async function setGlobalRecvMessageOpt(opt: 0 | 1 | 2): Promise<void> {
  await rest.setGlobalRecvMessageOpt(opt);
}

/** OpenIM: GetGlobalRecvMessageOpt */
export async function getGlobalRecvMessageOpt(): Promise<0 | 1 | 2> {
  return rest.getGlobalRecvMessageOpt();
}

export { getCurrentUser, updateCurrentUser } from "../core/rest";
