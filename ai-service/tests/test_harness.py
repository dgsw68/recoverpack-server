"""
Standalone before/after demo for app.harness.run_with_harness.

Simulates a flaky, slow Gemini-like call (no API key needed) to show:
  1. Retry: a call that fails twice then succeeds still returns a result,
     instead of the old behavior of falling straight to mock on first error.
  2. Timeout: a call that hangs forever is cut off instead of blocking forever.
  3. Concurrency: analyzing N "files" in parallel (asyncio.gather) is ~N times
     faster than the old sequential for-loop.

Run it directly:

    cd ai-service && python3 -m tests.test_harness
"""
import asyncio
import sys
import os
import time

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.harness import run_with_harness  # noqa: E402

CALL_LATENCY_SECONDS = 0.3


async def demo_retry():
    print("=== 1. RETRY: transient failure recovers instead of falling to mock ===")
    attempts = {"count": 0}

    async def flaky_call():
        attempts["count"] += 1
        await asyncio.sleep(0.05)
        if attempts["count"] < 3:
            raise RuntimeError("simulated transient Gemini error")
        return "OK: real Gemini result"

    result = await run_with_harness("demo_flaky", flaky_call, backoff_base_seconds=0.05)
    print(f"attempts={attempts['count']} result={result!r}")
    assert result == "OK: real Gemini result", "should recover after retries"
    print("OK: recovered on attempt 3 instead of giving up on attempt 1.\n")


async def demo_timeout():
    print("=== 2. TIMEOUT: a hanging call is cut off instead of blocking forever ===")

    async def hanging_call():
        await asyncio.sleep(10)
        return "should never get here"

    start = time.monotonic()
    result = await run_with_harness(
        "demo_hang", hanging_call, timeout_seconds=0.2, max_retries=0
    )
    elapsed = time.monotonic() - start
    print(f"result={result!r} elapsed={elapsed:.2f}s")
    assert result is None, "should give up and fall back to mock"
    assert elapsed < 1.0, "should not have waited for the full 10s hang"
    print("OK: gave up after the timeout instead of hanging.\n")


async def demo_concurrency():
    print("=== 3. CONCURRENCY: parallel analysis vs. old sequential for-loop ===")
    file_count = 5

    async def analyze_one(i):
        await asyncio.sleep(CALL_LATENCY_SECONDS)
        return f"file-{i}-result"

    start = time.monotonic()
    for i in range(file_count):
        await analyze_one(i)
    sequential_elapsed = time.monotonic() - start

    start = time.monotonic()
    await asyncio.gather(*(analyze_one(i) for i in range(file_count)))
    parallel_elapsed = time.monotonic() - start

    print(f"sequential (BEFORE): {sequential_elapsed:.2f}s for {file_count} files")
    print(f"parallel   (AFTER):  {parallel_elapsed:.2f}s for {file_count} files")
    assert parallel_elapsed < sequential_elapsed / 2, "parallel should be much faster"
    print("OK: parallel analysis is significantly faster.\n")


async def run():
    await demo_retry()
    await demo_timeout()
    await demo_concurrency()
    print("All harness demos passed.")


if __name__ == "__main__":
    asyncio.run(run())
