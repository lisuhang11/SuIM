"use client";

import React, { useEffect, useState } from "react";
import { Loader2, Mic, MicOff, Phone, PhoneOff, PhoneIncoming, X } from "lucide-react";
import { cn } from "@/lib/utils";
import UserAvatar from "../shared/UserAvatar";
import type { CallUiPhase } from "@/contexts/ChatContext";

type Props = {
  phase: CallUiPhase;
  peerName: string;
  peerAvatar: string;
  muted: boolean;
  busy: boolean;
  error: string | null;
  activeSince?: number;
  onAccept: () => void;
  onReject: () => void;
  onCancel: () => void;
  onHangup: () => void;
  onToggleMute: () => void;
  onDismissError: () => void;
};

function formatDuration(totalSec: number): string {
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export default function CallOverlay({
  phase,
  peerName,
  peerAvatar,
  muted,
  busy,
  error,
  activeSince,
  onAccept,
  onReject,
  onCancel,
  onHangup,
  onToggleMute,
  onDismissError,
}: Props) {
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (phase !== "active" || !activeSince) {
      setElapsed(0);
      return;
    }
    const tick = () => setElapsed(Math.max(0, Math.floor((Date.now() - activeSince) / 1000)));
    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [phase, activeSince]);

  if (phase === "idle" && !error) return null;

  const statusText =
    phase === "incoming"
      ? "来电…"
      : phase === "outgoing"
        ? "正在呼叫…"
        : phase === "active"
          ? formatDuration(elapsed)
          : "";

  const actionButton =
    "ui-press flex h-14 w-14 items-center justify-center rounded-full transition-colors disabled:opacity-50";

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink/50 p-4 sm:items-center">
      <div className="w-full max-w-sm overflow-hidden rounded-panel border border-edge bg-surface-elevated shadow-panel">
        {error ? (
          <div className="flex items-start justify-between gap-3 border-b border-edge bg-danger-soft px-4 py-3 text-sm text-danger">
            <span className="min-w-0 flex-1">{error}</span>
            <button
              type="button"
              onClick={onDismissError}
              className="flex-none rounded-control p-1 hover:bg-danger/10"
              aria-label="关闭"
            >
              <X className="h-4 w-4" strokeWidth={1.75} />
            </button>
          </div>
        ) : null}

        {phase !== "idle" ? (
          <div className="flex flex-col items-center px-6 pb-6 pt-8">
            <div className="relative">
              <UserAvatar src={peerAvatar} name={peerName} size="xl" className="h-24 w-24" />
              {phase === "incoming" ? (
                <span className="absolute -bottom-1 -right-1 flex h-8 w-8 items-center justify-center rounded-full bg-accent text-accent-fg">
                  <PhoneIncoming className="h-4 w-4 animate-pulse" strokeWidth={1.75} />
                </span>
              ) : null}
            </div>
            <h3 className="mt-5 truncate text-lg font-semibold text-ink">{peerName || "未知用户"}</h3>
            <p className="mt-1 text-sm text-ink-muted">{statusText}</p>

            <div className="mt-8 flex w-full items-center justify-center gap-4">
              {phase === "incoming" ? (
                <>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={onReject}
                    className={cn(actionButton, "bg-danger text-white hover:bg-danger/90")}
                    title="拒绝"
                  >
                    {busy ? (
                      <Loader2 className="h-6 w-6 animate-spin" strokeWidth={1.75} />
                    ) : (
                      <PhoneOff className="h-6 w-6" strokeWidth={1.75} />
                    )}
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={onAccept}
                    className={cn(actionButton, "bg-accent text-accent-fg hover:bg-accent-hover")}
                    title="接听"
                  >
                    {busy ? (
                      <Loader2 className="h-6 w-6 animate-spin" strokeWidth={1.75} />
                    ) : (
                      <Phone className="h-6 w-6" strokeWidth={1.75} />
                    )}
                  </button>
                </>
              ) : null}

              {phase === "outgoing" ? (
                <button
                  type="button"
                  disabled={busy}
                  onClick={onCancel}
                  className={cn(actionButton, "bg-danger text-white hover:bg-danger/90")}
                  title="取消"
                >
                  {busy ? (
                    <Loader2 className="h-6 w-6 animate-spin" strokeWidth={1.75} />
                  ) : (
                    <PhoneOff className="h-6 w-6" strokeWidth={1.75} />
                  )}
                </button>
              ) : null}

              {phase === "active" ? (
                <>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={onToggleMute}
                    className={cn(
                      actionButton,
                      muted ? "bg-surface-muted text-ink" : "bg-surface-muted text-ink hover:bg-edge"
                    )}
                    title={muted ? "取消静音" : "静音"}
                  >
                    {muted ? (
                      <MicOff className="h-6 w-6" strokeWidth={1.75} />
                    ) : (
                      <Mic className="h-6 w-6" strokeWidth={1.75} />
                    )}
                  </button>
                  <button
                    type="button"
                    disabled={busy}
                    onClick={onHangup}
                    className={cn(actionButton, "bg-danger text-white hover:bg-danger/90")}
                    title="挂断"
                  >
                    {busy ? (
                      <Loader2 className="h-6 w-6 animate-spin" strokeWidth={1.75} />
                    ) : (
                      <PhoneOff className="h-6 w-6" strokeWidth={1.75} />
                    )}
                  </button>
                </>
              ) : null}
            </div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
