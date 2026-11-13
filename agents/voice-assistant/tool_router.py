"""前台工具路由：同步工具阻塞当前回复；异步工具先回短答再后台出结果。"""

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
    """SYNC 宜短；ASYNC_* 只返回 ack，结果经回调送回。"""

    SYNC = "sync"
    ASYNC_QUERY = "async_query"
    ASYNC_EFFECT = "async_effect"


@dataclass
class ToolSpec:
    """一条可注册工具；handler 缺失时 invoke 返回未知提示。"""

    name: str
    description: str
    mode: ExecutionMode = ExecutionMode.SYNC
    handler: Callable[..., Awaitable[str]] | None = None


class ToolRouter:
    """按名调用工具；异步任务可 cancel_all。"""

    def __init__(self, on_async_result: AsyncResultHandler | None = None) -> None:
        self._specs: dict[str, ToolSpec] = {}
        self._tasks: dict[str, asyncio.Task[Any]] = {}
        self._on_async_result = on_async_result

    def register(self, spec: ToolSpec) -> None:
        """登记或覆盖同名工具。"""
        self._specs[spec.name] = spec

    def list_specs(self) -> list[ToolSpec]:
        """当前已登记工具快照。"""
        return list(self._specs.values())

    async def invoke(self, name: str, **kwargs: Any) -> str:
        """同步返回结果；异步立即返回口头 ack，结果走 on_async_result。"""
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
        """取消所有未完成的异步工具任务。"""
        for task in list(self._tasks.values()):
            task.cancel()
        self._tasks.clear()
