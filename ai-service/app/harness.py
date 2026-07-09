"""
Execution harness for outbound Gemini calls.

Wraps a single async call with:
  - a hard timeout (Gemini hanging must not hang the request)
  - exponential-backoff retry (transient errors shouldn't fall straight to mock)
  - structured telemetry logging (latency/retry/fallback per call)

Callers stay simple: pass a zero-arg async callable, get back its result or
None if every attempt failed (the caller's existing mock fallback kicks in).
"""
import asyncio
import logging
import time
from typing import Awaitable, Callable, Optional, TypeVar

logger = logging.getLogger("ai-service")

T = TypeVar("T")

DEFAULT_TIMEOUT_SECONDS = 20.0
DEFAULT_MAX_RETRIES = 2
DEFAULT_BACKOFF_BASE_SECONDS = 0.5


async def run_with_harness(
    label: str,
    call: Callable[[], Awaitable[T]],
    *,
    timeout_seconds: float = DEFAULT_TIMEOUT_SECONDS,
    max_retries: int = DEFAULT_MAX_RETRIES,
    backoff_base_seconds: float = DEFAULT_BACKOFF_BASE_SECONDS,
) -> Optional[T]:
    start = time.monotonic()
    retry_count = 0
    last_error: Optional[BaseException] = None

    for attempt in range(max_retries + 1):
        try:
            result = await asyncio.wait_for(call(), timeout=timeout_seconds)
            latency_ms = (time.monotonic() - start) * 1000
            logger.info(
                "harness call=%s ok=true attempt=%d retry_count=%d latency_ms=%.0f fallback_used=false",
                label, attempt + 1, retry_count, latency_ms,
            )
            return result
        except asyncio.TimeoutError as error:
            last_error = error
            logger.warning(
                "harness call=%s timeout attempt=%d timeout_s=%.1f",
                label, attempt + 1, timeout_seconds,
            )
        except Exception as error:
            last_error = error
            logger.warning(
                "harness call=%s error attempt=%d error=%s",
                label, attempt + 1, error,
            )

        if attempt < max_retries:
            retry_count += 1
            delay = backoff_base_seconds * (2 ** attempt)
            await asyncio.sleep(delay)

    latency_ms = (time.monotonic() - start) * 1000
    logger.error(
        "harness call=%s ok=false retry_count=%d latency_ms=%.0f fallback_used=true last_error=%s",
        label, retry_count, latency_ms, last_error,
    )
    return None
