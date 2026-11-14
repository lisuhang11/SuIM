from __future__ import annotations

import httpx

from voice_assistant.config import SiliconFlowConfig


def http_client(cfg: SiliconFlowConfig) -> httpx.AsyncClient:
    cfg.require_key()
    return httpx.AsyncClient(
        base_url=cfg.base_url.rstrip("/") + "/",
        headers={
            "Authorization": f"Bearer {cfg.api_key}",
            "User-Agent": "suim-voice-assistant/0.1",
        },
        timeout=httpx.Timeout(cfg.timeout_s, connect=10.0),
    )


def raise_for_api(resp: httpx.Response) -> None:
    if resp.is_success:
        return
    detail = resp.text
    try:
        body = resp.json()
        if isinstance(body, dict):
            detail = str(body.get("message") or body.get("error") or body)
    except Exception:
        pass
    raise RuntimeError(f"SiliconFlow HTTP {resp.status_code}: {detail[:500]}")
