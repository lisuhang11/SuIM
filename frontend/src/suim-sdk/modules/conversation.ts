// ============================================================
// SuIM SDK — Conversation module
// ============================================================
import * as rest from "../core/rest";
import { memoryCache } from "../cache/memory";
import type { Conversation } from "@/types";

/** OpenIM: GetAllConversationList — 经 BFF active-conversations 聚合 */
export async function getAllConversationList(): Promise<Conversation[]> {
  const { conversations } = await rest.getActiveConversations();
  memoryCache.conversations = conversations;
  return conversations;
}

/** OpenIM jssdk GetActiveConversations — 含 unreadTotal */
export async function getActiveConversations(count?: number) {
  const result = await rest.getActiveConversations(count);
  memoryCache.conversations = result.conversations;
  return result;
}

/** OpenIM: GetOneConversation */
export async function getOneConversation(conversationId: string): Promise<Conversation> {
  return rest.getConversation(conversationId);
}

export async function createPrivateConversation(userId: string): Promise<Conversation> {
  const conv = await rest.createPrivateConversation(userId);
  if (conv.conversationId) {
    memoryCache.conversations = [
      conv,
      ...memoryCache.conversations.filter((c) => c.conversationId !== conv.conversationId),
    ];
  }
  return conv;
}

export async function deleteConversation(conversationId: string): Promise<void> {
  await rest.deleteConversation(conversationId);
  memoryCache.conversations = memoryCache.conversations.filter(
    (c) => c.conversationId !== conversationId
  );
}

export async function setConversation(
  conversation: Conversation,
  patch: { isPinned?: boolean; isMuted?: boolean },
  ownerUserId: string
): Promise<void> {
  await rest.updateConversationSettings(conversation, patch, ownerUserId);
  memoryCache.conversations = memoryCache.conversations.map((c) =>
    c.conversationId === conversation.conversationId ? { ...c, ...patch } : c
  );
}

export {
  getConversations,
  getConversation,
  updateConversationSettings,
} from "../core/rest";
