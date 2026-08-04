// frontend/src/suim-sdk/modules/sync.ts
import { getIdb } from "../cache/idb";
import * as rest from "../core/rest";
import { wsManager } from "../listener/ws";
import type { Message } from "@/types";

const BATCH = 100;
const MAX_BATCHES_PER_CONV = 5;

export type SyncCompletedPayload = {
  conversationIds: string[];
  messagesByConversation: Record<string, Message[]>;
};

let running = false;
let pending = false;
let currentUserId: string | null = null;

function rangeSeqs(from: number, to: number): number[] {
  const out: number[] = [];
  for (let s = from; s <= to; s++) out.push(s);
  return out;
}

async function pullGap(
  conversationId: string,
  localMax: number,
  serverMax: number,
  hasReadSeq: number,
  userId: string,
  serverMin = 0
): Promise<Message[]> {
  const idb = getIdb(userId);
  const pulled: Message[] = [];
  let cursor = localMax;
  let batches = 0;
  while (cursor < serverMax && batches < MAX_BATCHES_PER_CONV) {
    if (currentUserId !== userId) return pulled;
    const end = Math.min(cursor + BATCH, serverMax);
    const seqs = rangeSeqs(cursor + 1, end);
    let msgs: Message[] = [];
    let attempt = 0;
    for (;;) {
      try {
        msgs = await rest.getMessagesBySeq(conversationId, seqs);
        break;
      } catch (err) {
        attempt++;
        if (attempt > 2) throw err;
        await new Promise((r) => setTimeout(r, 300 * 2 ** (attempt - 1)));
      }
    }
    if (currentUserId !== userId) return pulled;
    await idb.upsertMessages(msgs);
    pulled.push(...msgs);
    cursor = end;
    batches++;
    await idb.putConversationCursor({
      conversationId,
      maxSeq: cursor,
      minSeq: serverMin,
      hasReadSeq,
      updatedAt: Date.now(),
    });
    if (currentUserId !== userId) return pulled;
  }
  return pulled;
}

async function runOnce(userId: string): Promise<void> {
  if (currentUserId !== userId) return;
  const idb = getIdb(userId);
  const cursors = await idb.getAllConversationCursors();
  if (currentUserId !== userId) return;
  const localMax: Record<string, number> = {};
  const localHasRead: Record<string, number> = {};
  for (const c of cursors) {
    localMax[c.conversationId] = c.maxSeq;
    localHasRead[c.conversationId] = c.hasReadSeq;
  }

  const [serverBounds, hasRead] = await Promise.all([
    rest.getMaxAndMinSeqs(),
    rest.getHasReadAndMaxSeqs(),
  ]);
  if (currentUserId !== userId) return;
  await idb.setHasReadSeqs(hasRead);
  if (currentUserId !== userId) return;

  const changed: string[] = [];
  const messagesByConversation: Record<string, Message[]> = {};

  for (const [conversationId, bounds] of Object.entries(serverBounds)) {
    if (currentUserId !== userId) return;
    const max = bounds.maxSeq;
    const serverMin = bounds.minSeq ?? 0;
    const local = localMax[conversationId] ?? 0;
    if (max <= local) continue;
    const preservedHasRead = Math.max(
      localHasRead[conversationId] ?? 0,
      hasRead[conversationId]?.hasReadSeq ?? 0
    );
    try {
      const msgs = await pullGap(
        conversationId,
        local,
        max,
        preservedHasRead,
        userId,
        serverMin
      );
      if (currentUserId !== userId) return;
      if (msgs.length) {
        changed.push(conversationId);
        messagesByConversation[conversationId] = msgs;
      } else if (max > local) {
        // Still advance cursor toward server if pull returned empty (deleted/invisible).
        await idb.putConversationCursor({
          conversationId,
          maxSeq: Math.min(local + BATCH * MAX_BATCHES_PER_CONV, max),
          minSeq: serverMin,
          hasReadSeq: preservedHasRead,
          updatedAt: Date.now(),
        });
      }
    } catch (err) {
      console.warn("[MsgSyncer] gap pull failed", conversationId, err);
    }
  }

  if (currentUserId !== userId) return;
  wsManager.emit("sync.completed", {
    conversationIds: changed,
    messagesByConversation,
  });
}

export function setSyncUser(userId: string | null): void {
  currentUserId = userId;
}

export async function requestSync(): Promise<void> {
  const userId = currentUserId;
  if (!userId) return;
  if (running) {
    pending = true;
    return;
  }
  running = true;
  try {
    do {
      pending = false;
      await runOnce(userId);
    } while (pending);
  } finally {
    running = false;
  }
}
