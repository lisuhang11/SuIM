// frontend/src/suim-sdk/cache/idb.ts
import type { Contact, Group, GroupMemberInfo, Message } from "@/types";

const DB_VERSION = 4;

export type IdbConversationCursor = {
  conversationId: string;
  maxSeq: number;
  minSeq: number;
  hasReadSeq: number;
  updatedAt: number;
};

export type IdbSyncVersion = {
  table: string;
  entityId: string;
  version: number;
  versionId: string;
};

const FRIENDS_TABLE = "local_friends";
const JOINED_GROUPS_TABLE = "local_joined_groups";
const GROUP_MEMBERS_TABLE = "local_group_members";

function dbName(userId: string): string {
  return `suim-im-${userId}`;
}

function openDb(userId: string): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(dbName(userId), DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains("conversations")) {
        db.createObjectStore("conversations", { keyPath: "conversationId" });
      }
      if (!db.objectStoreNames.contains("messages")) {
        const store = db.createObjectStore("messages", { keyPath: "clientMsgId" });
        store.createIndex("byConvSeq", ["conversationId", "seq"], { unique: false });
      }
      if (!db.objectStoreNames.contains("friends")) {
        db.createObjectStore("friends", { keyPath: "userId" });
      }
      if (!db.objectStoreNames.contains("groups")) {
        db.createObjectStore("groups", { keyPath: "groupId" });
      }
      if (!db.objectStoreNames.contains("group_members")) {
        const store = db.createObjectStore("group_members", {
          keyPath: ["groupId", "userId"],
        });
        store.createIndex("byGroup", "groupId", { unique: false });
      }
      if (!db.objectStoreNames.contains("sync_versions")) {
        db.createObjectStore("sync_versions", { keyPath: ["table", "entityId"] });
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error ?? new Error("idb open failed"));
  });
}

function txDone(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error ?? new Error("idb tx failed"));
    tx.onabort = () => reject(tx.error ?? new Error("idb tx aborted"));
  });
}

export class SuimIdb {
  constructor(private userId: string) {}

  async getAllConversationCursors(): Promise<IdbConversationCursor[]> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("conversations", "readonly");
      const req = tx.objectStore("conversations").getAll();
      req.onsuccess = () => resolve((req.result ?? []) as IdbConversationCursor[]);
      req.onerror = () => reject(req.error);
    });
  }

  async putConversationCursor(cursor: IdbConversationCursor): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("conversations", "readwrite");
    const store = tx.objectStore("conversations");
    await new Promise<void>((resolve, reject) => {
      const getReq = store.get(cursor.conversationId);
      getReq.onsuccess = () => {
        const existing = getReq.result as IdbConversationCursor | undefined;
        store.put({
          ...cursor,
          maxSeq: Math.max(existing?.maxSeq ?? 0, cursor.maxSeq),
          hasReadSeq: Math.max(existing?.hasReadSeq ?? 0, cursor.hasReadSeq),
          minSeq: existing?.minSeq ?? cursor.minSeq,
        });
        resolve();
      };
      getReq.onerror = () => reject(getReq.error);
    });
    await txDone(tx);
  }

  async upsertMessages(messages: Message[]): Promise<void> {
    if (!messages.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction(["messages", "conversations"], "readwrite");
    const msgStore = tx.objectStore("messages");
    const convStore = tx.objectStore("conversations");
    const maxSeqByConv = new Map<string, number>();
    for (const m of messages) {
      if (!m.clientMsgId) {
        console.warn("[SuimIdb] upsertMessages: skipping message without clientMsgId", m);
        continue;
      }
      msgStore.put(m);
      const convId = m.conversationId;
      const seq = m.seq || 0;
      if (!convId || seq <= 0) continue;
      maxSeqByConv.set(convId, Math.max(maxSeqByConv.get(convId) ?? 0, seq));
    }
    if (maxSeqByConv.size > 0) {
      await new Promise<void>((resolve, reject) => {
        let pending = maxSeqByConv.size;
        for (const [convId, maxSeq] of maxSeqByConv) {
          const getReq = convStore.get(convId);
          getReq.onsuccess = () => {
            const prev = (getReq.result as IdbConversationCursor | undefined) ?? {
              conversationId: convId,
              maxSeq: 0,
              minSeq: 0,
              hasReadSeq: 0,
              updatedAt: 0,
            };
            prev.maxSeq = Math.max(prev.maxSeq, maxSeq);
            prev.updatedAt = Date.now();
            convStore.put(prev);
            pending--;
            if (pending === 0) resolve();
          };
          getReq.onerror = () => reject(getReq.error);
        }
      });
    }
    await txDone(tx);
  }

  async getMessagesByConversation(
    conversationId: string,
    limit = 50
  ): Promise<Message[]> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("messages", "readonly");
      const idx = tx.objectStore("messages").index("byConvSeq");
      const range = IDBKeyRange.bound([conversationId, 0], [conversationId, Number.MAX_SAFE_INTEGER]);
      const out: Message[] = [];
      const req = idx.openCursor(range, "prev");
      req.onsuccess = () => {
        const cursor = req.result;
        if (!cursor || out.length >= limit) {
          resolve(out.reverse());
          return;
        }
        out.push(cursor.value as Message);
        cursor.continue();
      };
      req.onerror = () => reject(req.error);
    });
  }

  async setHasReadSeqs(seqs: Record<string, { maxSeq: number; hasReadSeq: number }>): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("conversations", "readwrite");
    const store = tx.objectStore("conversations");
    for (const [conversationId, v] of Object.entries(seqs)) {
      const getReq = store.get(conversationId);
      getReq.onsuccess = () => {
        const prev = (getReq.result as IdbConversationCursor | undefined) ?? {
          conversationId,
          maxSeq: 0,
          minSeq: 0,
          hasReadSeq: 0,
          updatedAt: 0,
        };
        prev.hasReadSeq = Math.max(prev.hasReadSeq, v.hasReadSeq);
        // Do not overwrite local maxSeq with server here for gap logic; syncer advances after pull.
        prev.updatedAt = Date.now();
        store.put(prev);
      };
    }
    await txDone(tx);
  }

  async getAllFriends(): Promise<Contact[]> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("friends", "readonly");
      const req = tx.objectStore("friends").getAll();
      req.onsuccess = () => resolve((req.result ?? []) as Contact[]);
      req.onerror = () => reject(req.error);
    });
  }

  async replaceAllFriends(friends: Contact[]): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("friends", "readwrite");
    const store = tx.objectStore("friends");
    store.clear();
    for (const f of friends) {
      if (f?.userId) store.put(f);
    }
    await txDone(tx);
  }

  async putFriends(friends: Contact[]): Promise<void> {
    if (!friends.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("friends", "readwrite");
    const store = tx.objectStore("friends");
    for (const f of friends) {
      if (f?.userId) store.put(f);
    }
    await txDone(tx);
  }

  async deleteFriends(userIds: string[]): Promise<void> {
    if (!userIds.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("friends", "readwrite");
    const store = tx.objectStore("friends");
    for (const id of userIds) store.delete(id);
    await txDone(tx);
  }

  async getFriendSyncVersion(): Promise<IdbSyncVersion> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("sync_versions", "readonly");
      const req = tx.objectStore("sync_versions").get([FRIENDS_TABLE, this.userId]);
      req.onsuccess = () => {
        const v = req.result as IdbSyncVersion | undefined;
        resolve(
          v ?? {
            table: FRIENDS_TABLE,
            entityId: this.userId,
            version: 0,
            versionId: "",
          }
        );
      };
      req.onerror = () => reject(req.error);
    });
  }

  async putFriendSyncVersion(version: number, versionId: string): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("sync_versions", "readwrite");
    tx.objectStore("sync_versions").put({
      table: FRIENDS_TABLE,
      entityId: this.userId,
      version,
      versionId,
    } satisfies IdbSyncVersion);
    await txDone(tx);
  }

  async getAllJoinedGroups(): Promise<Group[]> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("groups", "readonly");
      const req = tx.objectStore("groups").getAll();
      req.onsuccess = () => resolve((req.result ?? []) as Group[]);
      req.onerror = () => reject(req.error);
    });
  }

  async replaceAllJoinedGroups(groups: Group[]): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("groups", "readwrite");
    const store = tx.objectStore("groups");
    store.clear();
    for (const g of groups) {
      if (g?.groupId) store.put(g);
    }
    await txDone(tx);
  }

  async putJoinedGroups(groups: Group[]): Promise<void> {
    if (!groups.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("groups", "readwrite");
    const store = tx.objectStore("groups");
    for (const g of groups) {
      if (g?.groupId) store.put(g);
    }
    await txDone(tx);
  }

  async deleteJoinedGroups(groupIds: string[]): Promise<void> {
    if (!groupIds.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("groups", "readwrite");
    const store = tx.objectStore("groups");
    for (const id of groupIds) store.delete(id);
    await txDone(tx);
  }

  async getJoinedGroupSyncVersion(): Promise<IdbSyncVersion> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("sync_versions", "readonly");
      const req = tx.objectStore("sync_versions").get([JOINED_GROUPS_TABLE, this.userId]);
      req.onsuccess = () => {
        const v = req.result as IdbSyncVersion | undefined;
        resolve(
          v ?? {
            table: JOINED_GROUPS_TABLE,
            entityId: this.userId,
            version: 0,
            versionId: "",
          }
        );
      };
      req.onerror = () => reject(req.error);
    });
  }

  async putJoinedGroupSyncVersion(version: number, versionId: string): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("sync_versions", "readwrite");
    tx.objectStore("sync_versions").put({
      table: JOINED_GROUPS_TABLE,
      entityId: this.userId,
      version,
      versionId,
    } satisfies IdbSyncVersion);
    await txDone(tx);
  }

  async getGroupMembers(groupId: string): Promise<GroupMemberInfo[]> {
    if (!groupId) return [];
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("group_members", "readonly");
      const idx = tx.objectStore("group_members").index("byGroup");
      const req = idx.getAll(groupId);
      req.onsuccess = () => resolve((req.result ?? []) as GroupMemberInfo[]);
      req.onerror = () => reject(req.error);
    });
  }

  async replaceGroupMembers(groupId: string, members: GroupMemberInfo[]): Promise<void> {
    if (!groupId) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("group_members", "readwrite");
    const store = tx.objectStore("group_members");
    const idx = store.index("byGroup");
    const existing = await new Promise<GroupMemberInfo[]>((resolve, reject) => {
      const req = idx.getAll(groupId);
      req.onsuccess = () => resolve((req.result ?? []) as GroupMemberInfo[]);
      req.onerror = () => reject(req.error);
    });
    for (const m of existing) {
      store.delete([groupId, m.userId]);
    }
    for (const m of members) {
      if (m?.userId) store.put({ ...m, groupId });
    }
    await txDone(tx);
  }

  async putGroupMembers(members: GroupMemberInfo[]): Promise<void> {
    if (!members.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("group_members", "readwrite");
    const store = tx.objectStore("group_members");
    for (const m of members) {
      if (m?.userId && m.groupId) store.put(m);
    }
    await txDone(tx);
  }

  async deleteGroupMembers(groupId: string, userIds: string[]): Promise<void> {
    if (!groupId || !userIds.length) return;
    const db = await openDb(this.userId);
    const tx = db.transaction("group_members", "readwrite");
    const store = tx.objectStore("group_members");
    for (const uid of userIds) store.delete([groupId, uid]);
    await txDone(tx);
  }

  async clearGroupMembers(groupId: string): Promise<void> {
    if (!groupId) return;
    await this.replaceGroupMembers(groupId, []);
  }

  async getGroupMemberSyncVersion(groupId: string): Promise<IdbSyncVersion> {
    const db = await openDb(this.userId);
    return new Promise((resolve, reject) => {
      const tx = db.transaction("sync_versions", "readonly");
      const req = tx.objectStore("sync_versions").get([GROUP_MEMBERS_TABLE, groupId]);
      req.onsuccess = () => {
        const v = req.result as IdbSyncVersion | undefined;
        resolve(
          v ?? {
            table: GROUP_MEMBERS_TABLE,
            entityId: groupId,
            version: 0,
            versionId: "",
          }
        );
      };
      req.onerror = () => reject(req.error);
    });
  }

  async putGroupMemberSyncVersion(
    groupId: string,
    version: number,
    versionId: string
  ): Promise<void> {
    const db = await openDb(this.userId);
    const tx = db.transaction("sync_versions", "readwrite");
    tx.objectStore("sync_versions").put({
      table: GROUP_MEMBERS_TABLE,
      entityId: groupId,
      version,
      versionId,
    } satisfies IdbSyncVersion);
    await txDone(tx);
  }
}

let active: SuimIdb | null = null;
let activeUserId: string | null = null;

export function getIdb(userId: string): SuimIdb {
  if (!active || activeUserId !== userId) {
    active = new SuimIdb(userId);
    activeUserId = userId;
  }
  return active;
}

export function clearIdbHandle(): void {
  active = null;
  activeUserId = null;
}
