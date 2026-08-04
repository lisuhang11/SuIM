// ============================================================
// SuIM SDK — Call control (REST /api/v1/calls)
// ============================================================
import type { ApiResponse } from "@/types";
import { apiRequest } from "../core/rest";

export type CallMediaType = "audio" | "video";
export type CallStatus = "ringing" | "accepted" | "active" | "ended";
export type CallEndReason =
  | "completed"
  | "rejected"
  | "cancelled"
  | "timeout"
  | "busy"
  | "unavailable";

export interface CallInfo {
  callId: string;
  conversationId: string;
  callerId: string;
  calleeId: string;
  mediaType: CallMediaType;
  status: CallStatus;
  endReason?: CallEndReason;
  roomName: string;
  startedAt?: number;
  answeredAt?: number;
  endedAt?: number;
  durationSec?: number;
}

export interface CallTipsPayload {
  callId: string;
  callerId?: string;
  calleeId?: string;
  mediaType?: CallMediaType;
  conversationId?: string;
  reason?: CallEndReason;
  durationSec?: number;
}

export interface InviteCallResult {
  call: CallInfo;
  token: string;
  livekitUrl: string;
}

export interface AcceptCallResult {
  call: CallInfo;
  token: string;
  livekitUrl: string;
}

export interface RefreshTokenResult {
  token: string;
  roomName: string;
  livekitUrl: string;
}

function livekitUrlFallback(): string {
  return process.env.NEXT_PUBLIC_LIVEKIT_URL || "ws://localhost:7880";
}

function unwrapData<T>(res: ApiResponse<T> | T): T {
  if (res && typeof res === "object" && "data" in (res as ApiResponse<T>)) {
    const wrapped = res as ApiResponse<T>;
    if (wrapped.data !== undefined && wrapped.data !== null) return wrapped.data;
  }
  return res as T;
}

function mapCallInfo(raw: Record<string, unknown>): CallInfo {
  const endReasonRaw = raw.end_reason ?? raw.endReason;
  return {
    callId: String(raw.call_id ?? raw.callId ?? ""),
    conversationId: String(raw.conversation_id ?? raw.conversationId ?? ""),
    callerId: String(raw.caller_id ?? raw.callerId ?? ""),
    calleeId: String(raw.callee_id ?? raw.calleeId ?? ""),
    mediaType: String(raw.media_type ?? raw.mediaType ?? "audio") as CallMediaType,
    status: String(raw.status ?? "ringing") as CallStatus,
    endReason: endReasonRaw
      ? (String(endReasonRaw) as CallEndReason)
      : undefined,
    roomName: String(raw.room_name ?? raw.roomName ?? ""),
    startedAt: Number(raw.started_at ?? raw.startedAt ?? 0) || undefined,
    answeredAt: Number(raw.answered_at ?? raw.answeredAt ?? 0) || undefined,
    endedAt: Number(raw.ended_at ?? raw.endedAt ?? 0) || undefined,
    durationSec: Number(raw.duration_sec ?? raw.durationSec ?? 0) || undefined,
  };
}

function mapInviteLike(raw: Record<string, unknown>): {
  call: CallInfo;
  token: string;
  livekitUrl: string;
} {
  const callRaw = (raw.call as Record<string, unknown> | undefined) || raw;
  return {
    call: mapCallInfo(callRaw),
    token: String(raw.token ?? ""),
    livekitUrl: String(raw.livekit_url ?? raw.livekitUrl ?? livekitUrlFallback()),
  };
}

export async function invite(
  calleeId: string,
  mediaType: CallMediaType = "audio"
): Promise<InviteCallResult> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>("/calls/invite", {
    method: "POST",
    body: JSON.stringify({ callee_id: calleeId, media_type: mediaType }),
  });
  return mapInviteLike(unwrapData(res) as Record<string, unknown>);
}

export async function accept(callId: string): Promise<AcceptCallResult> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}/accept`,
    { method: "POST", body: JSON.stringify({}) }
  );
  return mapInviteLike(unwrapData(res) as Record<string, unknown>);
}

export async function reject(callId: string): Promise<CallInfo> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}/reject`,
    { method: "POST", body: JSON.stringify({}) }
  );
  const d = unwrapData(res) as Record<string, unknown>;
  const callRaw = (d.call as Record<string, unknown> | undefined) || d;
  return mapCallInfo(callRaw);
}

export async function cancel(callId: string): Promise<CallInfo> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}/cancel`,
    { method: "POST", body: JSON.stringify({}) }
  );
  const d = unwrapData(res) as Record<string, unknown>;
  const callRaw = (d.call as Record<string, unknown> | undefined) || d;
  return mapCallInfo(callRaw);
}

export async function hangup(callId: string): Promise<CallInfo> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}/hangup`,
    { method: "POST", body: JSON.stringify({}) }
  );
  const d = unwrapData(res) as Record<string, unknown>;
  const callRaw = (d.call as Record<string, unknown> | undefined) || d;
  return mapCallInfo(callRaw);
}

export async function getCall(callId: string): Promise<CallInfo> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}`
  );
  const d = unwrapData(res) as Record<string, unknown>;
  const callRaw = (d.call as Record<string, unknown> | undefined) || d;
  return mapCallInfo(callRaw);
}

export async function refreshToken(callId: string): Promise<RefreshTokenResult> {
  const res = await apiRequest<ApiResponse<Record<string, unknown>>>(
    `/calls/${encodeURIComponent(callId)}/token`,
    { method: "POST", body: JSON.stringify({}) }
  );
  const d = unwrapData(res) as Record<string, unknown>;
  return {
    token: String(d.token ?? ""),
    roomName: String(d.room_name ?? d.roomName ?? ""),
    livekitUrl: String(d.livekit_url ?? d.livekitUrl ?? livekitUrlFallback()),
  };
}
