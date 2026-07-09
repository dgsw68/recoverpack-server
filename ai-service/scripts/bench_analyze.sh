#!/usr/bin/env bash
# Real before/after latency measurement for /internal/analyze-image.
# Run this against a live `uvicorn app.main:app` with GEMINI_API_KEY set,
# once on current code (after) and once with the harness changes stashed
# (before), then compare the printed total time.
#
# Usage:
#   cd ai-service && ./scripts/bench_analyze.sh
#   ./scripts/bench_analyze.sh http://localhost:8000   # custom host

set -euo pipefail

BASE_URL="${1:-http://localhost:8000}"

PAYLOAD='{
  "project_id": "harness-bench",
  "files": [
    { "id": "1", "file_name": "floor_flooding_1.jpg", "file_type": "image", "file_url": "https://example.com/1.jpg", "mime_type": "image/jpeg" },
    { "id": "2", "file_name": "wall_damage_1.jpg", "file_type": "image", "file_url": "https://example.com/2.jpg", "mime_type": "image/jpeg" },
    { "id": "3", "file_name": "appliance_damage_1.jpg", "file_type": "image", "file_url": "https://example.com/3.jpg", "mime_type": "image/jpeg" },
    { "id": "4", "file_name": "furniture_damage_1.jpg", "file_type": "image", "file_url": "https://example.com/4.jpg", "mime_type": "image/jpeg" },
    { "id": "5", "file_name": "receipt_1.jpg", "file_type": "receipt", "file_url": "https://example.com/5.jpg", "mime_type": "image/jpeg" },
    { "id": "6", "file_name": "disaster_alert_1.jpg", "file_type": "alert", "file_url": "https://example.com/6.jpg", "mime_type": "image/jpeg" },
    { "id": "7", "file_name": "estimate_1.jpg", "file_type": "estimate", "file_url": "https://example.com/7.jpg", "mime_type": "image/jpeg" },
    { "id": "8", "file_name": "floor_flooding_2.jpg", "file_type": "image", "file_url": "https://example.com/8.jpg", "mime_type": "image/jpeg" }
  ]
}'

echo "Health check:"
curl -s "${BASE_URL}/health"
echo -e "\n"

echo "Sending 8-file analyze-image request to ${BASE_URL} ..."
curl -s -o /tmp/bench_response.json -w "\nHTTP %{http_code}  |  total time: %{time_total}s\n" \
  -X POST "${BASE_URL}/internal/analyze-image" \
  -H "Content-Type: application/json" \
  -d "${PAYLOAD}"

echo "Response saved to /tmp/bench_response.json"
