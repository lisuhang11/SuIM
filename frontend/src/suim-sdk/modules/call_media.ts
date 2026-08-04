// ============================================================
// SuIM SDK — LiveKit media (Phase A: audio only)
// ============================================================
import { Room } from "livekit-client";

let room: Room | null = null;

export async function connect(opts: { url: string; token: string }): Promise<Room> {
  await disconnect();
  const next = new Room({ adaptiveStream: true, dynacast: true });
  await next.connect(opts.url, opts.token);
  await next.localParticipant.setMicrophoneEnabled(true);
  room = next;
  return next;
}

export async function setMicEnabled(enabled: boolean): Promise<void> {
  if (!room) return;
  await room.localParticipant.setMicrophoneEnabled(enabled);
}

export function isMicEnabled(): boolean {
  return room?.localParticipant.isMicrophoneEnabled ?? false;
}

export async function disconnect(): Promise<void> {
  if (!room) return;
  try {
    await room.disconnect();
  } finally {
    room = null;
  }
}

export function getRoom(): Room | null {
  return room;
}
