# 黑名单前端界面设计

**Date:** 2026-07-30  
**Status:** Approved (placement A / scope A / entry A / approach A)

## Goal

补齐通讯录「黑名单」管理界面；拉黑入口保持会话更多菜单。suim-sdk 黑名单 API 对齐 OpenIM `GetBlackList` / `AddBlack` / `RemoveBlack` 写法（内存 cache，无 SQLite）。

## UX

- 通讯录第三 Tab：`好友 | 新的朋友 | 黑名单`
- `BlacklistPanel`：列表（头像、昵称、拉黑时间）+ 取消拉黑（confirm）
- 分页：`offset/limit`，默认 limit=50，支持加载更多
- 空态：「暂无黑名单用户」
- 不改 FriendProfilePanel；不把 blacklist 拉进 ChatContext
- **拉黑不删除好友关系**（好友列表仍保留该用户）

## SDK

- `memoryCache.blacks`
- `getBlackList` → 写 cache
- `addBlack(userId, ex?)` → REST；更新 blacks（不改 friends）
- `removeBlack(userId)` → REST；从 blacks 移除
- 错误文案：`user is already blocked` / `user is not blocked`

## Backend note

`GetBlockedUsersResp` 使用 `repeated BlackInfo blacks`（非旧版 `blocked_user_ids`）。网关与 relation 须使用同一版 proto。`BlockUser` 不再调用 `DeleteFriendPair`。

## Files

- `frontend/src/suim-sdk/cache/memory.ts`
- `frontend/src/suim-sdk/modules/relation.ts`
- `frontend/src/suim-sdk/core/rest.ts`
- `frontend/src/components/chat/BlacklistPanel.tsx`
- `frontend/src/components/chat/FriendsPanel.tsx`
- `frontend/src/components/chat/ChatHeader.tsx`
- `services/relation/internal/service/relation.go`
