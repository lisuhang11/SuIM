#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

if [[ ! -d .venv ]]; then
  uv venv .venv
fi
uv pip install -r requirements.txt --python .venv/bin/python >/dev/null

# Load LiveKit + Redis settings from rtc.yaml without printing secrets.
eval "$(.venv/bin/python - <<'PY'
from pathlib import Path
import yaml, shlex
cfg = yaml.safe_load(Path("/home/lisuhang/SuIM/services/rtc/etc/rtc.yaml").read_text())
lk = cfg.get("livekit") or {}
redis = cfg.get("redis") or {}
agents = cfg.get("agents") or {}
password = redis.get("password") or ""
addr = (redis.get("address") or ["127.0.0.1:6379"])[0]
auth = f":{password}@" if password else ""
print(f"export LIVEKIT_URL={shlex.quote(lk.get('url') or 'ws://127.0.0.1:7880')}")
print(f"export LIVEKIT_API_KEY={shlex.quote(lk.get('key') or 'suim')}")
print(f"export LIVEKIT_API_SECRET={shlex.quote(lk.get('secret') or '')}")
print(f"export REDIS_URL={shlex.quote('redis://' + auth + addr + '/0')}")
print(f"export AGENT_STREAM={shlex.quote(agents.get('stream') or 'suim:agent:dispatch')}")
PY
)"

exec .venv/bin/python worker.py
