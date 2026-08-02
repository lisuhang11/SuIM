// ============================================================
// SuIM SDK — in-memory cache (OpenIM-style L1)
// ============================================================
import type {
  User,
  Contact,
  Group,
  GroupMemberInfo,
  Conversation,
  BlacklistEntry,
} from "@/types";

class MemoryCache {
  selfUser: User | null = null;
  users = new Map<string, User>();
  friends: Contact[] = [];
  /** OpenIM LocalBlack 的迷你 SDK 等价物 */
  blacks: BlacklistEntry[] = [];
  groups: Group[] = [];
  /** per groupId member list */
  groupMembers = new Map<string, GroupMemberInfo[]>();
  conversations: Conversation[] = [];

  setSelf(user: User | null): void {
    this.selfUser = user;
    if (user?.userId) this.users.set(user.userId, user);
  }

  putUsers(list: User[]): void {
    for (const u of list) {
      if (u.userId) this.users.set(u.userId, u);
    }
  }

  getUsers(ids: string[]): { hit: User[]; miss: string[] } {
    const hit: User[] = [];
    const miss: string[] = [];
    for (const id of ids) {
      const u = this.users.get(id);
      if (u) hit.push(u);
      else miss.push(id);
    }
    return { hit, miss };
  }

  getGroup(groupId: string): Group | undefined {
    if (!groupId) return undefined;
    return this.groups.find((g) => g.groupId === groupId);
  }

  putGroup(group: Group): void {
    if (!group?.groupId) return;
    this.groups = [group, ...this.groups.filter((g) => g.groupId !== group.groupId)];
  }

  putGroups(list: Group[]): void {
    for (const g of list) this.putGroup(g);
  }

  removeGroup(groupId: string): void {
    if (!groupId) return;
    this.groups = this.groups.filter((g) => g.groupId !== groupId);
    this.groupMembers.delete(groupId);
  }

  getGroupMembers(groupId: string): GroupMemberInfo[] {
    if (!groupId) return [];
    return this.groupMembers.get(groupId) ?? [];
  }

  setGroupMembers(groupId: string, members: GroupMemberInfo[]): void {
    if (!groupId) return;
    this.groupMembers.set(groupId, members);
  }

  getGroups(ids: string[]): { hit: Group[]; miss: string[] } {
    const hit: Group[] = [];
    const miss: string[] = [];
    for (const id of ids) {
      const g = this.getGroup(id);
      if (g) hit.push(g);
      else miss.push(id);
    }
    return { hit, miss };
  }

  clear(): void {
    this.selfUser = null;
    this.users.clear();
    this.friends = [];
    this.blacks = [];
    this.groups = [];
    this.groupMembers.clear();
    this.conversations = [];
  }
}

export const memoryCache = new MemoryCache();
