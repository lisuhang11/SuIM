// ============================================================
// SuIM ID helpers — conversationId vs groupId
// 约定：群会话 conversation_id = gid_<group_id>（对齐服务端 groupChatID）
// ============================================================

/** 裸 groupId → 群会话 ID */
export function groupConversationId(groupId: string): string {
  if (!groupId) return "";
  return groupId.startsWith("gid_") ? groupId : `gid_${groupId}`;
}

/** 会话 ID / 误传的 gid_ 前缀 → 裸 groupId（供 /groups/:id 等接口） */
export function parseGroupId(idOrConversationId: string): string {
  if (!idOrConversationId) return "";
  return idOrConversationId.startsWith("gid_")
    ? idOrConversationId.slice(4)
    : idOrConversationId;
}
