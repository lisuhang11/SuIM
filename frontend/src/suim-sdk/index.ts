// ============================================================
// SuIM TypeScript Mini SDK — public entry
// OpenIM 风格：UI 统一 `import { IMSDK } from "@/suim-sdk"`
// ============================================================
export { SuIMSDK, getSDK, createSDK } from "./client";
export type { InitConfig } from "./client";
export { wsManager } from "./listener/ws";
export { memoryCache } from "./cache/memory";
export { getIdb, clearIdbHandle } from "./cache/idb";
export type { IdbConversationCursor } from "./cache/idb";
export { toMessage, getUsersBatch } from "./core/rest";
export { groupConversationId, parseGroupId } from "./core/ids";

import { getSDK } from "./client";

/** 全局单例，对齐 OpenIM `const IMSDK = getSDK()` */
export const IMSDK = getSDK();

// Domain modules (also available via IMSDK / getSDK())
export * as user from "./modules/user";
export * as relation from "./modules/relation";
export * as friendSync from "./modules/friend_sync";
export * as presence from "./modules/presence";
export * as group from "./modules/group";
export * as groupSync from "./modules/group_sync";
export * as memberSync from "./modules/member_sync";
export * as conversation from "./modules/conversation";
export * as message from "./modules/message";
export * as sync from "./modules/sync";
export * as file from "./modules/file";
export * as call from "./modules/call";
export * as callMedia from "./modules/call_media";

// Flat re-exports for convenient named imports
export {
  login,
  register,
  getCurrentUser,
  logout,
  changePassword,
  getConversations,
  getConversation,
  createPrivateConversation,
  createGroup,
  deleteConversation,
  markAsRead,
  revokeMessage,
  sendMessage,
  updateConversationSettings,
  getMessages,
  getContacts,
  searchUsers,
  sendFriendRequest,
  respondFriendRequest,
  getIncomingRequests,
  getOutgoingRequests,
  getUnhandledRequestCount,
  deleteFriend,
  updateFriend,
  getBlackList,
  blockUser,
  unblockUser,
  getGroups,
  updateGroupInfo,
  dismissGroup,
  transferGroupOwner,
  inviteToGroup,
  kickGroupMember,
  quitGroup,
  getGroupMembers,
  setGroupMute,
  setMemberMute,
  applyToJoinGroup,
  getPendingApplications,
  getMyApplications,
  handleApplication,
  getUnhandledGroupApplicationCount,
  updateCurrentUser,
  uploadFile,
  uploadAvatar,
  resolveAvatarURL,
  getFileDownloadURL,
} from "./core/rest";

// 带 L1 memoryCache 的群资料查询（对齐 OpenIM GetSpecifiedGroupsInfo）
export { getGroupInfo, getGroupsInfo } from "./modules/group";
