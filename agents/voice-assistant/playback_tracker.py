"""Moss-Live-style playback progress tracking for LiveKit AgentSession.

Agent publishes run lifecycle on topic ``suim.agent.playback``.
Client reports actual hear-progress on the same topic; the tracker keeps the
latest ``played_ms`` so interrupted replies can resume from a rough cut point.
"""

from __future__ import annotations

import json
import logging
import time
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

LOG = logging.getLogger("suim-agent.playback")

TOPIC = "suim.agent.playback"

PublishFn = Callable[[str, str], Awaitable[None]]


@dataclass
class PlaybackRun:
    run_id: str
    text: str = ""
    started_at: float = field(default_factory=time.time)
    played_ms: float = 0.0
    interrupted: bool = False
    completed: bool = False


class PlaybackTracker:
    def __init__(self, publish: PublishFn | None = None) -> None:
        self._publish = publish
        self._runs: dict[str, PlaybackRun] = {}
        self._active_id: str | None = None
        self.last_interrupt: PlaybackRun | None = None

    def bind_publish(self, publish: PublishFn) -> None:
        self._publish = publish

    async def _emit(self, payload: dict[str, Any]) -> None:
        if self._publish is None:
            return
        try:
            await self._publish(TOPIC, json.dumps(payload, ensure_ascii=False))
        except Exception as err:  # noqa: BLE001
            LOG.warning("playback emit failed: %s", err)

    async def on_speech_started(self, run_id: str, *, text: str = "") -> None:
        run = PlaybackRun(run_id=run_id, text=text or "")
        self._runs[run_id] = run
        self._active_id = run_id
        await self._emit(
            {
                "type": "response.audio.started",
                "run_id": run_id,
                "text": run.text,
            }
        )

    async def on_speech_finished(self, run_id: str, *, interrupted: bool) -> None:
        run = self._runs.get(run_id)
        if run is None:
            run = PlaybackRun(run_id=run_id)
            self._runs[run_id] = run
        if interrupted:
            if run.played_ms <= 0:
                run.played_ms = max(0.0, (time.time() - run.started_at) * 1000.0)
            run.interrupted = True
            self.last_interrupt = run
            await self._emit(
                {
                    "type": "response.audio.stop",
                    "run_id": run_id,
                    "played_ms": run.played_ms,
                    "reason": "interrupted",
                }
            )
            await self._emit(
                {
                    "type": "response.interrupted",
                    "run_id": run_id,
                    "played_ms": run.played_ms,
                    "text": run.text,
                }
            )
        else:
            run.completed = True
            await self._emit({"type": "response.audio.done", "run_id": run_id})
            await self._emit({"type": "playback.completed", "run_id": run_id})
        if self._active_id == run_id:
            self._active_id = None

    def ingest_client_event(self, raw: str | bytes) -> None:
        try:
            if isinstance(raw, bytes):
                raw = raw.decode("utf-8")
            body = json.loads(raw)
        except Exception:  # noqa: BLE001
            return
        if not isinstance(body, dict):
            return
        etype = body.get("type")
        run_id = body.get("run_id")
        if not isinstance(run_id, str) or not run_id:
            return
        run = self._runs.get(run_id)
        if run is None:
            run = PlaybackRun(run_id=run_id)
            self._runs[run_id] = run
        played = body.get("played_ms")
        if isinstance(played, (int, float)) and played >= 0:
            run.played_ms = max(run.played_ms, float(played))
        if etype in {"playback.chunk.completed", "playback.progress"}:
            return
        if etype == "playback.completed":
            run.completed = True
        elif etype == "playback.stopped":
            run.interrupted = True
            self.last_interrupt = run

    def set_text(self, run_id: str, text: str) -> None:
        run = self._runs.get(run_id)
        if run is not None and text:
            run.text = text

    def resume_hint(self) -> str:
        run = self.last_interrupt
        if run is None or not run.text:
            return ""
        ms = int(run.played_ms)
        return (
            f"上一轮助手回复曾被用户打断（约已播 {ms} ms）。"
            f"原文大致是：{run.text[:240]}。"
            "若用户要你继续，从断点附近接着说，不要从头完整重念；"
            "若用户换了话题，直接跟新话题。"
        )
