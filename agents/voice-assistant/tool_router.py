"""Foreground tool routing: sync vs async (Moss-Live style).

Sync tools block the current reply (keep them <~2s).
Async tools should return a short ack string immediately and schedule work
off the audio path; call ``complete_async`` when the result is ready so the
session can speak again without holding the mic loop.
"""

from __future__ import annotations

import asyncio
import logging
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from enum import Enum
from typing import Any

LOG = logging.getLogger("suim-agent.tools")

AsyncResultHandler = Callable[[str, str], Awaitable[None]]


class ExecutionMode(str, Enum):
    SYNC = "sync"
    ASYNC_QUERY = "async_query"
    ASYNC_EFFECT = "async_effect"


@dataclass
class ToolSpec:
    name: str
    description: str
    mode: ExecutionMode = ExecutionMode.SYNC
    handler: Callable[..., Awaitable[str]] | None = None


class ToolRouter:
    def __init__(self, on_async_result: AsyncResultHandler | None = None) -> None:
        self._specs: dict[str, ToolSpec] = {}
        self._tasks: dict[str, asyncio.Task[Any]] = {}
        self._on_async_result = on_async_result

    def register(self, spec: ToolSpec) -> None:
        self._specs[spec.name] = spec

    def list_specs(self) -> list[ToolSpec]:
        return list(self._specs.values())

    async def invoke(self, name: str, **kwargs: Any) -> str:
        spec = self._specs.get(name)
        if spec is None or spec.handler is None:
            return f"未知工具: {name}"
        if spec.mode == ExecutionMode.SYNC:
            return await spec.handler(**kwargs)

        call_id = uuid.uuid4().hex
        task = asyncio.create_task(
            self._run_async(call_id, spec, kwargs),
            name=f"tool-{name}-{call_id[:8]}",
        )
        self._tasks[call_id] = task
        verb = "查询" if spec.mode == ExecutionMode.ASYNC_QUERY else "处理"
        return f"好的，我先去{verb}，有结果再告诉你。"

    async def _run_async(self, call_id: str, spec: ToolSpec, kwargs: dict[str, Any]) -> None:
        assert spec.handler is not None
        try:
            result = await spec.handler(**kwargs)
        except Exception as err:  # noqa: BLE001
            LOG.exception("async tool %s failed", spec.name)
            result = f"{spec.name} 失败: {err}"
        finally:
            self._tasks.pop(call_id, None)
        if self._on_async_result is not None:
            await self._on_async_result(call_id, result)

    def cancel_all(self) -> None:
        for task in list(self._tasks.values()):
            task.cancel()
        self._tasks.clear()
