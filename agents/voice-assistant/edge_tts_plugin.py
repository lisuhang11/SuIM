"""Minimal edge-tts backend for LiveKit Agents (no paid TTS key)."""

from __future__ import annotations

import io
import uuid
from dataclasses import dataclass

import av
import edge_tts
from livekit.agents import APIConnectOptions, tts
from livekit.agents.types import DEFAULT_API_CONNECT_OPTIONS

SAMPLE_RATE = 24000
NUM_CHANNELS = 1


@dataclass
class _Opts:
    voice: str
    sample_rate: int


class TTS(tts.TTS):
    def __init__(self, *, voice: str = "zh-CN-XiaoxiaoNeural", sample_rate: int = SAMPLE_RATE) -> None:
        super().__init__(
            capabilities=tts.TTSCapabilities(streaming=False),
            sample_rate=sample_rate,
            num_channels=NUM_CHANNELS,
        )
        self._opts = _Opts(voice=voice, sample_rate=sample_rate)

    @property
    def model(self) -> str:
        return "edge-tts"

    @property
    def provider(self) -> str:
        return "edge-tts"

    def synthesize(
        self, text: str, *, conn_options: APIConnectOptions = DEFAULT_API_CONNECT_OPTIONS
    ) -> tts.ChunkedStream:
        return _ChunkedStream(tts=self, input_text=text, conn_options=conn_options)


class _ChunkedStream(tts.ChunkedStream):
    def __init__(self, *, tts: TTS, input_text: str, conn_options: APIConnectOptions) -> None:
        super().__init__(tts=tts, input_text=input_text, conn_options=conn_options)
        self._tts = tts

    async def _run(self, output_emitter: tts.AudioEmitter) -> None:
        text = (self.input_text or "").strip()
        output_emitter.initialize(
            request_id=uuid.uuid4().hex,
            sample_rate=self._tts.sample_rate,
            num_channels=NUM_CHANNELS,
            mime_type="audio/pcm",
        )
        if not text:
            output_emitter.flush()
            return

        communicate = edge_tts.Communicate(text, self._tts._opts.voice)
        mp3 = bytearray()
        async for chunk in communicate.stream():
            if chunk["type"] == "audio":
                mp3.extend(chunk["data"])
        if not mp3:
            output_emitter.flush()
            return

        pcm = _mp3_to_pcm(bytes(mp3), self._tts.sample_rate)
        if pcm:
            output_emitter.push(pcm)
        output_emitter.flush()


def _mp3_to_pcm(mp3: bytes, sample_rate: int) -> bytes:
    container = av.open(io.BytesIO(mp3), format="mp3")
    resampler = av.audio.resampler.AudioResampler(format="s16", layout="mono", rate=sample_rate)
    pcm = bytearray()
    for frame in container.decode(audio=0):
        for out in resampler.resample(frame):
            pcm.extend(bytes(out.planes[0]))
    for out in resampler.resample(None):
        pcm.extend(bytes(out.planes[0]))
    return bytes(pcm)
