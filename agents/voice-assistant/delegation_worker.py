"""Background client-delegation scaffold (Moss-Live FG/BG split).

Foreground session enqueues a DelegationJob; this helper runs it without
blocking audio I/O and reports progress/results via callbacks.
"""

from __future__ import annotations

import asyncio
import logging
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any, Literal

LOG = logging.getLogger("suim-agent.delegation")

Purpose = Literal["task", "reasoning"]
ProgressFn = Callable[[str, bool], Awaitable[None]]  # text, spoken
ResultFn = Callable[[str, str], Awaitable[None]]  # delegation_id, result


@dataclass
class DelegationJob:
    objective: str
    purpose: Purpose = "task"
    context: list[dict[str, str]] = field(default_factory=list)
    id: str = field(default_factory=lambda: uuid.uuid4().hex)


class DelegationWorker:
    def __init__(
        self,
        *,
        runner: Callable[[DelegationJob, ProgressFn, asyncio.Event], Awaitable[str]] | None = None,
        on_progress: ProgressFn | None = None,
        on_result: ResultFn | None = None,
    ) -> None:
        self._runner = runner
        self._on_progress = on_progress
        self._on_result = on_result
        self._jobs: dict[str, asyncio.Task[Any]] = {}
        self._cancels: dict[str, asyncio.Event] = {}

    def enqueue(self, job: DelegationJob) -> str:
        cancel = asyncio.Event()
        self._cancels[job.id] = cancel
        task = asyncio.create_task(self._run(job, cancel), name=f"delegation-{job.id[:8]}")
        self._jobs[job.id] = task
        return job.id

    async def _run(self, job: DelegationJob, cancel: asyncio.Event) -> None:
        async def progress(text: str, spoken: bool = False) -> None:
            if self._on_progress is not None:
                await self._on_progress(text, spoken)

        try:
            if self._runner is None:
                result = f"（未配置后台执行器）目标：{job.objective}"
            else:
                result = await self._runner(job, progress, cancel)
            if cancel.is_set():
                return
            if self._on_result is not None:
                await self._on_result(job.id, result)
        except asyncio.CancelledError:
            raise
        except Exception:  # noqa: BLE001
            LOG.exception("delegation %s failed", job.id)
            if self._on_result is not None and not cancel.is_set():
                await self._on_result(job.id, f"后台任务失败: {job.objective}")
        finally:
            self._jobs.pop(job.id, None)
            self._cancels.pop(job.id, None)

    def cancel(self, delegation_id: str) -> bool:
        ev = self._cancels.get(delegation_id)
        task = self._jobs.get(delegation_id)
        if ev is None and task is None:
            return False
        if ev is not None:
            ev.set()
        if task is not None:
            task.cancel()
        return True

    def cancel_all(self) -> None:
        for did in list(self._cancels):
            self.cancel(did)
