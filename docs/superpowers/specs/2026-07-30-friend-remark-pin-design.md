# Friend remark & pin — Design Spec

**Date:** 2026-07-30  
**Status:** Approved for planning  
**Approach:** A — thin stack extension (proto → relation → gateway → SDK → FriendProfilePanel)

## Goal

Let users set a **remark** and **pin** on a single friend relationship, surface those fields in the friend list API, and edit them from a **friend profile panel**. Display names prefer remark over nickname.

## Non-goals

- Batch `UpdateFriends` / OpenIM wrapperspb
- Separate `SetFriendRemark` RPC
- Friend `ex` / `add_source` mutation via this API
- OpenIM-style friend-info change notifications / incremental sync
- Editing the peer’s account profile (nickname/avatar remain user service)

## Decisions (from brainstorming)

| Topic | Choice |
|-------|--------|
| Update API | Single-friend `UpdateFriend` |
| Fields | `remark` + `is_pinned` only |
| Partial update | Proto3 `optional`; unset = no change; `""` clears remark; `false` unpins |
| List API | Extend `GetFriends` to return `FriendInfo` (replace ID-only shape) |
| Frontend entry | Friend profile panel (not list overflow menu) |
| Display name | `remark \|\| nickname` in list / private chat title |
| Search | Match remark, nickname, username |

## Backend

### Proto (`proto/relation.proto`)

```protobuf
message FriendInfo {
  string friend_user_id = 1;
  string remark         = 2;
  bool   is_pinned      = 3;
  int64  create_time    = 4;
}

message GetFriendsResp {
  repeated FriendInfo friends = 1;
  int32 total = 2;
}

message UpdateFriendReq {
  string owner_user_id  = 1;
  string friend_user_id = 2;
  optional string remark    = 3;
  optional bool   is_pinned = 4;
}

message UpdateFriendResp {}

rpc UpdateFriend(UpdateFriendReq) returns (UpdateFriendResp);
```

`GetFriends` response **drops** `friend_ids`; clients must use `friends`.

### Storage

- Table `friend` already has `remark`, `is_pinned` (and unused `ex` / `add_source`).
- `ListFriends` already orders `is_pinned DESC, create_time DESC` — keep.
- Add repository `UpdateFriend(ctx, ownerUserID, friendUserID, fields map)` (or typed patch).
- On accept-friend, continue creating rows with empty remark / unpinned.

### Service rules

1. Auth: `owner_user_id` must equal caller; gateway injects from JWT and does not trust client owner.
2. Friend must exist; else NotFound / “not friends”.
3. If neither `remark` nor `is_pinned` is set → InvalidArgument.
4. Remark max length: **64** runes (reject over limit).
5. Last-write-wins; no optimistic locking.
6. No relation Redis cache today — no invalidation step unless added later.

### Gateway

| Method | Path | Body / notes |
|--------|------|----------------|
| GET | `/api/v1/relations/friends` | `{ friends, total }` |
| PUT | `/api/v1/relations/friends/:friend_id` | `{ remark?: string, is_pinned?: boolean }` |

## Frontend

### Types / SDK

- `Contact`: add `remark: string`, `isPinned: boolean`.
- Effective display helper: `remark || displayName` (nickname).
- `getFriendList`: parse `friends[]`, batch-load users, merge.
- `updateFriend(friendId, { remark?, isPinned? })` → PUT above.

### UI

- **FriendsPanel**: row click opens `FriendProfilePanel`; existing message button still starts chat.
- **FriendProfilePanel**: avatar, effective name, original nickname, `@username`, remark editor, pin toggle, “发消息”.
- After save: refresh contacts (and open conversation title if needed) so pin order and names update.
- Toasts on failure (not friend, validation, network).

### Compatibility

Breaking change for `GET /friends` payload (`friend_ids` → `friends`). Update SuIM TS SDK and any `services/api` re-exports in the same change; no long dual-shape period.

## Testing (plan phase will detail)

- Unit/service: patch remark only; pin only; clear remark; empty patch rejected; non-friend; overlong remark.
- Gateway/SDK smoke: list returns fields; PUT then GET reflects.
- UI: profile edit updates list title and pin order.

## Implementation order

1. Proto + regenerate  
2. Repo / service / gRPC handler  
3. Gateway routes  
4. SDK + types  
5. FriendProfilePanel + FriendsPanel wiring + display-name helpers  
