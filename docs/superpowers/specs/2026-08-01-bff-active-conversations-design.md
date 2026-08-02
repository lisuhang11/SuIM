# SuIM BFF Active Conversations Design

**Date:** 2026-08-01  
**Status:** Approved (user chose full OpenIM-aligned B + conversation→message RPC + compatible Conversation shape)

## Goal

One HTTP call returns a renderable conversation list: sorted active conversations, last message, unread, title/avatar. Align with OpenIM jssdk / msg.GetActiveConversation / GetLastMessage.

## Architecture

```
SDK getAllConversationList
  → POST /api/v1/bff/active-conversations  (apigateway BFF)
  → conversation + message + user + relation + group RPCs

conversation.ListLatestMsgs → message.GetLastMessage (no direct msg_info SQL)
```

## message RPC additions

- `GetActiveConversation(conversation_ids, limit?) → [{conversation_id, max_seq, last_time}]`
- `GetLastMessage(user_id, conversation_ids) → map[id]MsgData`

## HTTP

`POST /api/v1/bff/active-conversations` body `{count}`  
Response: `{ conversations: Conversation[], unread_total }` (existing Conversation fields)

## SDK

`getAllConversationList` calls BFF; ChatContext keeps friends/groups fetch for contacts; enrichConversations as fallback only.
