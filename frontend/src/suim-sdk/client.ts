// ============================================================
// SuIMSDK — OpenIM-style client façade (Promise API)
// ============================================================
import { setApiBase, getApiBase } from "./core/rest";
import { setWsBase, getWsBase, wsManager } from "./listener/ws";
import { memoryCache } from "./cache/memory";
import { clearIdbHandle } from "./cache/idb";
import * as user from "./modules/user";
import * as relation from "./modules/relation";
import * as group from "./modules/group";
import * as conversation from "./modules/conversation";
import * as message from "./modules/message";
import * as file from "./modules/file";
import * as sync from "./modules/sync";
import { setFriendSyncUser, incrSyncFriends } from "./modules/friend_sync";
import { setJoinedGroupSyncUser, incrSyncJoinedGroups } from "./modules/group_sync";
import {
  setGroupMemberSyncUser,
  incrSyncGroupMembers,
  incrSyncJoinedGroupMembers,
} from "./modules/member_sync";
import * as presence from "./modules/presence";
import * as call from "./modules/call";
import * as callMedia from "./modules/call_media";
import { setToken as persistToken, getCachedUser } from "@/services/storage";
import type { User, WsMessage, WsMessageType } from "@/types";

export type InitConfig = {
  apiAddr?: string;
  wsAddr?: string;
};

export class SuIMSDK {
  private initialized = false;
  private loggedIn = false;
  private syncHooked = false;

  /** OpenIM: InitSDK */
  initSDK(config: InitConfig = {}): boolean {
    if (config.apiAddr) setApiBase(config.apiAddr);
    if (config.wsAddr) setWsBase(config.wsAddr);
    if (!this.syncHooked) {
      this.syncHooked = true;
      wsManager.onStatusChange((connected) => {
        if (connected && this.loggedIn) {
          void sync.requestSync();
          void incrSyncFriends().then(() => {
            void presence.syncFriendsAccountsPresence();
          });
          void incrSyncJoinedGroups().then((groups) => {
            void incrSyncJoinedGroupMembers(groups.map((g) => g.groupId));
          });
        }
      });
      wsManager.on("friend.sync", () => {
        if (this.loggedIn) void incrSyncFriends();
      });
      wsManager.on("friend.synced", () => {
        if (this.loggedIn) void presence.syncFriendsAccountsPresence();
      });
      wsManager.on("group.sync", (msg) => {
        if (!this.loggedIn) return;
        void incrSyncJoinedGroups();
        const groupId = String(
          (msg.payload as { groupId?: string } | undefined)?.groupId ?? ""
        );
        if (groupId) void incrSyncGroupMembers(groupId);
      });
      wsManager.on("group.member.sync", (msg) => {
        if (!this.loggedIn) return;
        const groupId = String(
          (msg.payload as { groupId?: string } | undefined)?.groupId ?? ""
        );
        if (groupId) void incrSyncGroupMembers(groupId);
      });
    }
    this.initialized = true;
    return true;
  }

  get isInitialized(): boolean {
    return this.initialized;
  }

  get isLoggedIn(): boolean {
    return this.loggedIn;
  }

  get apiAddr(): string {
    return getApiBase();
  }

  get wsAddr(): string {
    return getWsBase();
  }

  // ---------- Auth / User ----------
  login = async (...args: Parameters<typeof user.login>) => {
    const res = await user.login(...args);
    if (res.token) {
      persistToken(res.token);
      this.loggedIn = true;
      if (res.user?.userId) {
        sync.setSyncUser(res.user.userId);
        setFriendSyncUser(res.user.userId);
        setJoinedGroupSyncUser(res.user.userId);
        setGroupMemberSyncUser(res.user.userId);
        void incrSyncFriends().then(() => {
          void presence.syncFriendsAccountsPresence();
        });
        void incrSyncJoinedGroups().then((groups) => {
          void incrSyncJoinedGroupMembers(groups.map((g) => g.groupId));
        });
      }
      wsManager.connect();
    }
    return res;
  };

  register = async (...args: Parameters<typeof user.register>) => {
    const res = await user.register(...args);
    if (res.token) {
      persistToken(res.token);
      this.loggedIn = true;
      if (res.user?.userId) {
        sync.setSyncUser(res.user.userId);
        setFriendSyncUser(res.user.userId);
        setJoinedGroupSyncUser(res.user.userId);
        setGroupMemberSyncUser(res.user.userId);
      }
    }
    return res;
  };

  logout = async () => {
    try {
      await user.logout();
    } finally {
      this.loggedIn = false;
      wsManager.disconnect();
      memoryCache.clear();
      sync.setSyncUser(null);
      setFriendSyncUser(null);
      setJoinedGroupSyncUser(null);
      setGroupMemberSyncUser(null);
      clearIdbHandle();
    }
  };

  changePassword = user.changePassword;
  getSelfUserInfo = user.getSelfUserInfo;
  setSelfInfo = user.setSelfInfo;
  getUsersInfo = user.getUsersInfo;
  searchUsers = user.searchUsers;
  setGlobalRecvMessageOpt = user.setGlobalRecvMessageOpt;
  getGlobalRecvMessageOpt = user.getGlobalRecvMessageOpt;
  getCurrentUser = user.getCurrentUser;
  updateCurrentUser = user.updateCurrentUser;

  // ---------- Relation ----------
  getFriendList = relation.getFriendList;
  addFriend = relation.addFriend;
  acceptFriendApplication = relation.acceptFriendApplication;
  refuseFriendApplication = relation.refuseFriendApplication;
  getFriendApplicationListAsRecipient = relation.getFriendApplicationListAsRecipient;
  getFriendApplicationListAsApplicant = relation.getFriendApplicationListAsApplicant;
  getFriendApplicationUnhandledCount = relation.getFriendApplicationUnhandledCount;
  deleteFriend = relation.deleteFriend;
  updateFriend = relation.updateFriend;
  incrSyncFriends = relation.incrSyncFriends;
  getBlackList = relation.getBlackList;
  addBlack = relation.addBlack;
  removeBlack = relation.removeBlack;
  getContacts = relation.getContacts;
  subscribeUsersStatus = presence.subscribeUsersStatus;
  getUsersOnlineStatus = presence.getUsersOnlineStatus;
  sendFriendRequest = relation.sendFriendRequest;
  respondFriendRequest = relation.respondFriendRequest;
  getIncomingRequests = relation.getIncomingRequests;
  getOutgoingRequests = relation.getOutgoingRequests;
  getUnhandledRequestCount = relation.getUnhandledRequestCount;

  // ---------- Group ----------
  createGroup = group.createGroup;
  groupConversationId = group.groupConversationId;
  parseGroupId = group.parseGroupId;
  getJoinedGroupList = group.getJoinedGroupList;
  incrSyncJoinedGroups = group.incrSyncJoinedGroups;
  incrSyncGroupMembers = group.incrSyncGroupMembers;
  getGroupsInfo = group.getGroupsInfo;
  setGroupInfo = group.setGroupInfo;
  getGroupMemberList = group.getGroupMemberList;
  inviteUserToGroup = group.inviteUserToGroup;
  kickGroupMember = group.kickGroupMember;
  quitGroup = group.quitGroup;
  dismissGroup = group.dismissGroup;
  transferGroupOwner = group.transferGroupOwner;
  changeGroupMute = group.changeGroupMute;
  changeGroupMemberMute = group.changeGroupMemberMute;
  joinGroup = group.joinGroup;
  getGroupApplicationListAsRecipient = group.getGroupApplicationListAsRecipient;
  getGroupApplicationListAsApplicant = group.getGroupApplicationListAsApplicant;
  acceptGroupApplication = group.acceptGroupApplication;
  refuseGroupApplication = group.refuseGroupApplication;
  getGroupApplicationUnhandledCount = group.getGroupApplicationUnhandledCount;
  getGroups = group.getGroups;
  getGroupInfo = group.getGroupInfo;
  updateGroupInfo = group.updateGroupInfo;
  getGroupMembers = group.getGroupMembers;
  getPendingApplications = group.getPendingApplications;
  getMyApplications = group.getMyApplications;
  handleApplication = group.handleApplication;
  applyToJoinGroup = group.applyToJoinGroup;
  setGroupMute = group.setGroupMute;
  setMemberMute = group.setMemberMute;
  inviteToGroup = group.inviteToGroup;

  // ---------- Conversation ----------
  getAllConversationList = conversation.getAllConversationList;
  getActiveConversations = conversation.getActiveConversations;
  getOneConversation = conversation.getOneConversation;
  createPrivateConversation = conversation.createPrivateConversation;
  deleteConversation = conversation.deleteConversation;
  setConversation = conversation.setConversation;
  getConversations = conversation.getConversations;
  getConversation = conversation.getConversation;
  updateConversationSettings = conversation.updateConversationSettings;

  // ---------- Message ----------
  getAdvancedHistoryMessageList = message.getAdvancedHistoryMessageList;
  sendMessage = message.sendMessage;
  markConversationMessageAsRead = message.markConversationMessageAsRead;
  revokeMessage = message.revokeMessage;
  getMessages = message.getMessages;
  markAsRead = message.markAsRead;
  toMessage = message.toMessage;

  // ---------- File ----------
  uploadFile = file.uploadFile;
  uploadAvatar = file.uploadAvatar;
  resolveAvatarURL = file.resolveAvatarURL;
  getFileDownloadURL = file.getFileDownloadURL;

  // ---------- Call (Phase A voice) ----------
  inviteCall = call.invite;
  acceptCall = call.accept;
  rejectCall = call.reject;
  cancelCall = call.cancel;
  hangupCall = call.hangup;
  getCall = call.getCall;
  refreshCallToken = call.refreshToken;
  connectCallMedia = callMedia.connect;
  setCallMicEnabled = callMedia.setMicEnabled;
  isCallMicEnabled = callMedia.isMicEnabled;
  disconnectCallMedia = callMedia.disconnect;

  // ---------- Connection ----------
  connect = () => {
    wsManager.connect();
  };

  disconnect = () => {
    wsManager.disconnect();
  };

  on = (type: WsMessageType, handler: (msg: WsMessage) => void) => wsManager.on(type, handler);

  onStatusChange = (handler: (connected: boolean) => void) =>
    wsManager.onStatusChange(handler);

  sendWs = (type: WsMessageType, payload: unknown) => wsManager.send(type, payload);

  /** Alias for UI that expects wsManager.send */
  send = (type: WsMessageType, payload: unknown) => wsManager.send(type, payload);

  get connected(): boolean {
    return wsManager.connected;
  }

  /** Mark session restored (token already in storage). */
  markLoggedIn(value = true): void {
    this.loggedIn = value;
    if (value) {
      const cached = getCachedUser<User>();
      if (cached?.userId) {
        sync.setSyncUser(cached.userId);
        setFriendSyncUser(cached.userId);
        setJoinedGroupSyncUser(cached.userId);
        setGroupMemberSyncUser(cached.userId);
        void incrSyncFriends();
        void incrSyncJoinedGroups().then((groups) => {
          void incrSyncJoinedGroupMembers(groups.map((g) => g.groupId));
        });
      }
    } else {
      sync.setSyncUser(null);
      setFriendSyncUser(null);
      setJoinedGroupSyncUser(null);
      setGroupMemberSyncUser(null);
    }
  }
}

let singleton: SuIMSDK | null = null;

export function getSDK(): SuIMSDK {
  if (!singleton) {
    singleton = new SuIMSDK();
    singleton.initSDK();
  }
  return singleton;
}

export function createSDK(config?: InitConfig): SuIMSDK {
  const sdk = new SuIMSDK();
  sdk.initSDK(config);
  singleton = sdk;
  return sdk;
}
