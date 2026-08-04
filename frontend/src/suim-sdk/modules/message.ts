// ============================================================
// SuIM SDK — Message module
// ============================================================
import * as rest from "../core/rest";
import type { Message } from "@/types";

/** OpenIM: GetAdvancedHistoryMessageList (simplified) */
export async function getAdvancedHistoryMessageList(
  conversationId: string,
  params?: { before?: string; limit?: number }
): Promise<Message[]> {
  return rest.getMessages(conversationId, params);
}

export async function sendMessage(payload: {
  clientMsgId: string;
  conversationId: string;
  sessionType: number;
  groupId?: string;
  recvId?: string;
  recvUserIds?: string[];
  contentType: number;
  content: string;
  senderNickname?: string;
  senderFaceUrl?: string;
}): Promise<{ serverMsgId: string; clientMsgId: string; seq: number; sendTime: number }> {
  return rest.sendMessage(payload);
}

/** OpenIM: MarkConversationMessageAsRead */
export async function markConversationMessageAsRead(
  conversationId: string,
  seq: number
): Promise<void> {
  await rest.markAsRead(conversationId, seq);
}

/** OpenIM: RevokeMessage */
export async function revokeMessage(
  conversationId: string,
  clientMsgId: string
): Promise<void> {
  await rest.revokeMessage(conversationId, clientMsgId);
}

export async function getMaxSeqs(conversationIds?: string[]) {
  return rest.getMaxSeqs(conversationIds);
}

export async function getMaxAndMinSeqs(conversationIds?: string[]) {
  return rest.getMaxAndMinSeqs(conversationIds);
}

export async function getHasReadAndMaxSeqs(conversationIds?: string[]) {
  return rest.getHasReadAndMaxSeqs(conversationIds);
}

export async function getMessagesBySeq(conversationId: string, seqs: number[]) {
  return rest.getMessagesBySeq(conversationId, seqs);
}

export { getMessages, markAsRead, toMessage } from "../core/rest";
