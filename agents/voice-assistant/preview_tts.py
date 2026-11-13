#!/usr/bin/env python3
"""Synthesize a short preview clip with edge-tts (SuIM-owned, no Moss)."""

from __future__ import annotations

import argparse
import asyncio
import sys

import edge_tts


async def _run(voice: str, text: str, out: str) -> None:
    communicate = edge_tts.Communicate(text, voice)
    await communicate.save(out)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--voice", required=True)
    parser.add_argument("--text", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()
    try:
        asyncio.run(_run(args.voice.strip(), args.text.strip(), args.out))
    except Exception as err:  # noqa: BLE001
        print(f"preview_tts failed: {err}", file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
