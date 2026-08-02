# SuIM TypeScript Mini SDK

Client kernel for the Next.js frontend. Lives at `frontend/src/suim-sdk`.

Aligned with OpenIM web usage:

```ts
import { IMSDK } from "@/suim-sdk";
// or: import { getSDK } from "@/suim-sdk"; const IMSDK = getSDK();

await IMSDK.login({ username, password });
const me = await IMSDK.getSelfUserInfo();
const friends = await IMSDK.getFriendList();
```

## Modules

| Module | OpenIM-style APIs |
|--------|-------------------|
| User | `login` `getSelfUserInfo`（batch） `setSelfInfo` `getUsersInfo`（batch） `searchUsers` `setGlobalRecvMessageOpt` `getGlobalRecvMessageOpt` |
| Relation | `getFriendList` `addFriend` `getBlackList` `addBlack` `removeBlack` … |
| Group | `createGroup` `getGroupInfo`/`getGroupsInfo` `getJoinedGroupList` `setGroupInfo` … |
| Conversation | `getAllConversationList` `createPrivateConversation` `setConversation` … |
| Message | `getAdvancedHistoryMessageList` `sendMessage` `markConversationMessageAsRead` |
| File | `uploadFile` `uploadAvatar` `getFileDownloadURL` |
| Conn | `connect` `disconnect` `on` `onStatusChange` `send` |

UI should import `@/suim-sdk` only.
