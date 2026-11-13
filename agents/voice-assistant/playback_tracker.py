"""跟踪助手播报进度：服务端发 lifecycle，客户端回传 played_ms，供打断后续说。"""

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
    """单次助手播报；played_ms 以客户端回执为准，缺省时用墙上时间估算。"""

    run_id: str
    text: str = ""
    started_at: float = field(default_factory=time.time)
    played_ms: float = 0.0
    interrupted: bool = False
    completed: bool = False


class PlaybackTracker:
    """维护活跃/最近打断的播报，并向 suim.agent.playback 发事件。"""

    def __init__(self, publish: PublishFn | None = None) -> None:
        self._publish = publish
        self._runs: dict[str, PlaybackRun] = {}
        self._active_id: str | None = None
        self.last_interrupt: PlaybackRun | None = None

    def bind_publish(self, publish: PublishFn) -> None:
        """绑定经 data channel 发 JSON 的回调；可晚于构造注入。"""
        self._publish = publish

    async def _emit(self, payload: dict[str, Any]) -> None:
        if self._publish is None:
            return
        try:
            await self._publish(TOPIC, json.dumps(payload, ensure_ascii=False))
        except Exception as err:  # noqa: BLE001
            LOG.warning("playback emit failed: %s", err)

    async def on_speech_started(self, run_id: str, *, text: str = "") -> None:
        """标记一轮播报开始并通知前端。"""
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
        """标记结束或打断；打断时写入 last_interrupt 供续说。"""
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
        """吸收客户端进度/停止事件；坏 JSON 静默忽略。"""
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
        """在合成完成后再补全文案；空串不覆盖。"""
        run = self._runs.get(run_id)
        if run is not None and text:
            run.text = text

    def resume_hint(self) -> str:
        """给模型的续说提示；无打断记录时返回空串。"""
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
