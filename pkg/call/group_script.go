package call

// groupHelpers is the shared body of the group-room scripts.
// Keys are built from the prefix, matching the private-call scripts.
const groupHelpers = `
local function clearOne(prefix, callID, uid)
  local key = prefix .. "busy:" .. uid
  if redis.call("GET", key) == callID then
    redis.call("DEL", key)
  end
end
local function ids(memberKey, want)
  local out = {}
  local all = redis.call("HGETALL", memberKey)
  for i = 1, #all, 2 do
    if all[i + 1] == want then
      table.insert(out, all[i])
    end
  end
  return out
end
local function others(caller, joined)
  local n = 0
  for _, uid in ipairs(joined) do
    if uid ~= caller then n = n + 1 end
  end
  return n
end
local function shouldEnd(caller, actor, joined, ringing)
  if #joined == 0 then return true end
  local rest = others(caller, joined)
  if actor == caller and rest == 0 then return true end
  if #ringing == 0 and rest == 0 then return true end
  return false
end
local function finish(prefix, callKey, memberKey, gring, groupID, callID)
  local all = redis.call("HGETALL", memberKey)
  for i = 1, #all, 2 do
    local uid, st = all[i], all[i + 1]
    if st == "joined" or st == "ringing" then
      clearOne(prefix, callID, uid)
      if st == "ringing" then
        redis.call("ZREM", gring, callID .. ":" .. uid)
      end
    end
  end
  redis.call("DEL", callKey, memberKey, prefix .. "group:" .. groupID)
end
local function touch(prefix, callKey, memberKey, groupID, callID, liveTTL, joined)
  redis.call("EXPIRE", callKey, liveTTL)
  redis.call("EXPIRE", memberKey, liveTTL)
  redis.call("EXPIRE", prefix .. "group:" .. groupID, liveTTL)
  for _, uid in ipairs(joined) do
    local key = prefix .. "busy:" .. uid
    if redis.call("GET", key) == callID then
      redis.call("EXPIRE", key, liveTTL)
    end
  end
end
local function pack(status, groupID, caller, media, joined, ringing)
  local out = {status, groupID or "", caller or "", media or "", ":joined"}
  for _, uid in ipairs(joined) do table.insert(out, uid) end
  table.insert(out, ":ringing")
  for _, uid in ipairs(ringing) do table.insert(out, uid) end
  return out
end
local function load(callKey)
  if redis.call("EXISTS", callKey) == 0 then return nil, "missing" end
  if redis.call("HGET", callKey, "kind") ~= "group" then return nil, "state" end
  return {
    caller = redis.call("HGET", callKey, "caller"),
    groupID = redis.call("HGET", callKey, "group"),
    media = redis.call("HGET", callKey, "media"),
  }, nil
end
`

const inviteGroupScript = `
local caller, groupID, callID, media = ARGV[1], ARGV[2], ARGV[3], ARGV[4]
local deadline, liveTTL, ringTTL, prefix = tonumber(ARGV[5]), tonumber(ARGV[6]), tonumber(ARGV[7]), ARGV[8]
local hostBusy = prefix .. "busy:" .. caller
if not redis.call("SET", hostBusy, callID, "EX", liveTTL, "NX") then
  return {"self"}
end
if not redis.call("SET", KEYS[1], callID, "EX", liveTTL, "NX") then
  redis.call("DEL", hostBusy)
  return {"group"}
end
redis.call("HSET", KEYS[2], "kind", "group", "caller", caller, "group", groupID, "media", media, "state", "live")
redis.call("EXPIRE", KEYS[2], liveTTL)
redis.call("HSET", KEYS[3], caller, "joined")
redis.call("EXPIRE", KEYS[3], liveTTL)
local invited, busy = {}, {}
for i = 9, #ARGV do
  local uid = ARGV[i]
  if uid ~= "" and uid ~= caller then
    if redis.call("SET", prefix .. "busy:" .. uid, callID, "EX", ringTTL, "NX") then
      redis.call("HSET", KEYS[3], uid, "ringing")
      redis.call("ZADD", KEYS[4], deadline, callID .. ":" .. uid)
      table.insert(invited, uid)
    else
      table.insert(busy, uid)
    end
  end
end
local out = {"ok", groupID, caller, media, ":joined", caller, ":ringing"}
for _, uid in ipairs(invited) do table.insert(out, uid) end
table.insert(out, ":busy")
for _, uid in ipairs(busy) do table.insert(out, uid) end
return out
`

const acceptGroupScript = groupHelpers + `
local userID, callID, liveTTL, prefix = ARGV[1], ARGV[2], tonumber(ARGV[3]), ARGV[5]
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem then return {problem} end
local status = redis.call("HGET", memberKey, userID)
if status ~= "ringing" then
  if not status then return {"forbidden"} end
  return {"state"}
end
redis.call("HSET", memberKey, userID, "joined")
redis.call("ZREM", gring, callID .. ":" .. userID)
redis.call("EXPIRE", prefix .. "busy:" .. userID, liveTTL)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
touch(prefix, callKey, memberKey, room.groupID, callID, liveTTL, joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const rejectGroupScript = groupHelpers + `
local userID, callID, liveTTL, prefix = ARGV[1], ARGV[2], tonumber(ARGV[3]), ARGV[5]
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem == "missing" then
  redis.call("ZREM", gring, callID .. ":" .. userID)
end
if problem then return {problem} end
local status = redis.call("HGET", memberKey, userID)
if status ~= "ringing" then
  redis.call("ZREM", gring, callID .. ":" .. userID)
  if not status then return {"forbidden"} end
  return {"state"}
end
redis.call("HSET", memberKey, userID, "rejected")
clearOne(prefix, callID, userID)
redis.call("ZREM", gring, callID .. ":" .. userID)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
if shouldEnd(room.caller, userID, joined, ringing) then
  finish(prefix, callKey, memberKey, gring, room.groupID, callID)
  return pack("ended", room.groupID, room.caller, room.media, joined, ringing)
end
touch(prefix, callKey, memberKey, room.groupID, callID, liveTTL, joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const leaveGroupScript = groupHelpers + `
local userID, callID, liveTTL, prefix = ARGV[1], ARGV[2], tonumber(ARGV[3]), ARGV[5]
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem then return {problem} end
local status = redis.call("HGET", memberKey, userID)
if status ~= "joined" then
  if not status then return {"forbidden"} end
  return {"state"}
end
redis.call("HSET", memberKey, userID, "left")
clearOne(prefix, callID, userID)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
if shouldEnd(room.caller, userID, joined, ringing) then
  finish(prefix, callKey, memberKey, gring, room.groupID, callID)
  return pack("ended", room.groupID, room.caller, room.media, joined, ringing)
end
touch(prefix, callKey, memberKey, room.groupID, callID, liveTTL, joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const enterGroupScript = groupHelpers + `
local userID, callID, liveTTL, prefix = ARGV[1], ARGV[2], tonumber(ARGV[3]), ARGV[5]
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem then return {problem} end
local status = redis.call("HGET", memberKey, userID)
if status == "joined" then return {"state"} end
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
if status ~= "ringing" and (#joined + #ringing) >= 9 then return {"full"} end
if status == "ringing" then
  redis.call("ZREM", gring, callID .. ":" .. userID)
  redis.call("EXPIRE", prefix .. "busy:" .. userID, liveTTL)
else
  if not redis.call("SET", prefix .. "busy:" .. userID, callID, "EX", liveTTL, "NX") then
    return {"self"}
  end
end
redis.call("HSET", memberKey, userID, "joined")
joined = ids(memberKey, "joined")
ringing = ids(memberKey, "ringing")
touch(prefix, callKey, memberKey, room.groupID, callID, liveTTL, joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const timeoutGroupScript = groupHelpers + `
local userID, callID, liveTTL, prefix = ARGV[1], ARGV[2], tonumber(ARGV[3]), ARGV[5]
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem == "missing" then
  redis.call("ZREM", gring, callID .. ":" .. userID)
end
if problem then return {problem} end
local status = redis.call("HGET", memberKey, userID)
if status ~= "ringing" then
  redis.call("ZREM", gring, callID .. ":" .. userID)
  return {"state"}
end
redis.call("HSET", memberKey, userID, "timeout")
clearOne(prefix, callID, userID)
redis.call("ZREM", gring, callID .. ":" .. userID)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
if shouldEnd(room.caller, userID, joined, ringing) then
  finish(prefix, callKey, memberKey, gring, room.groupID, callID)
  return pack("ended", room.groupID, room.caller, room.media, joined, ringing)
end
touch(prefix, callKey, memberKey, room.groupID, callID, liveTTL, joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const restoreRingScript = groupHelpers + `
local userID, callID, ringTTL, prefix, deadline = ARGV[1], ARGV[2], tonumber(ARGV[4]), ARGV[5], tonumber(ARGV[6])
local callKey, memberKey, gring = KEYS[1], KEYS[2], KEYS[3]
local room, problem = load(callKey)
if problem then return {problem} end
if redis.call("HGET", memberKey, userID) ~= "joined" then return {"state"} end
redis.call("HSET", memberKey, userID, "ringing")
redis.call("ZADD", gring, deadline, callID .. ":" .. userID)
redis.call("EXPIRE", prefix .. "busy:" .. userID, ringTTL)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
touch(prefix, callKey, memberKey, room.groupID, callID, tonumber(ARGV[3]), joined)
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`

const undoJoinScript = groupHelpers + `
local userID, callID, prefix = ARGV[1], ARGV[2], ARGV[5]
local callKey, memberKey = KEYS[1], KEYS[2]
local room, problem = load(callKey)
if problem then return {problem} end
if redis.call("HGET", memberKey, userID) ~= "joined" then return {"state"} end
redis.call("HSET", memberKey, userID, "left")
clearOne(prefix, callID, userID)
local joined = ids(memberKey, "joined")
local ringing = ids(memberKey, "ringing")
return pack("ok", room.groupID, room.caller, room.media, joined, ringing)
`
