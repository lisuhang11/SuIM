# SuIM TypeScript Mini SDK Design

**Date:** 2026-07-30  
**Location:** `frontend/src/suim-sdk`  
**Status:** Approved (user chose B + full module coverage)

## Goal

Make `suim-sdk` the client kernel: all REST/WS I/O and domain APIs live here. React contexts/panels call the SDK (or thin `services/*` re-exports), not raw fetch.

## Scope (phase 1)

Mirror **already-implemented frontend modules** against **real SuIM gateway** APIs:

| Module | OpenIM-style names (Promise façade) | Backend |
|--------|-------------------------------------|---------|
| User / Auth | `login`, `register`, `logout`, `getSelfUserInfo`, `setSelfInfo`, `getUsersInfo`, `searchUsers`, `changePassword` | `/users/*` |
| Relation | `getFriendList`, `addFriend`, `acceptFriendApplication`, `refuseFriendApplication`, `getFriendApplicationListAsRecipient/Applicant`, `getFriendApplicationUnhandledCount`, `deleteFriend` | `/relations/*` |
| Group | `getJoinedGroupList`, `getGroupsInfo`, `setGroupInfo`, members/mute/invite/kick/quit/dismiss/transfer, applications | `/groups/*` |
| Conversation | `getAllConversationList`, `getOneConversation`, `createPrivate/Group`, `deleteConversation`, `setConversation` | `/conversations/*` |
| Message | `getAdvancedHistoryMessageList`, `sendMessage`, `markConversationMessageAsRead` | `/messages/*` |
| File | `uploadFile`, `uploadAvatar`, `resolveAvatarURL`, `getFileDownloadURL` | `/files/*` + avatar routes |
| Conn | `connect` / `disconnect` / listeners (WS) | Message gateway WS |

## Architecture

```
UI / AuthContext / ChatContext
        ↓
   SuIMSDK (singleton)
        ↓
  modules/*  →  core/http + listener/ws  →  SuIM Gateway
        ↓
  in-memory cache (self user, batch users)
```

- **Not** WASM / Go bridge in phase 1.
- Keep UI types in `@/types`; SDK maps snake_case ↔ camelCase.
- `services/api.ts` / `services/websocket.ts` become **compat re-exports** so panels keep working during migration.
- Session token remains `localStorage` keys `suim_token` / `suim_user` (same as today).

## Non-goals (phase 1)

- SQLite / IndexedDB full offline sync
- Callback-style `operationID` (use Promise + optional listeners)
- APIs SuIM backend does not expose

## Migration

1. Move transport + domain into `suim-sdk`.
2. Wire AuthContext / ChatContext to `getSDK()`.
3. Leave panel imports on `api` re-exports until optionally switched.
