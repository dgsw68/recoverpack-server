"""
Standalone before/after demo for app.grounding.ground_text.

Runs without GEMINI_API_KEY (falls back to the regex fact-check only, since
embeddings need the API). Run it directly:

    cd ai-service && python3 -m tests.test_grounding
"""
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from app.grounding import ground_text  # noqa: E402

EVIDENCE_CAPTIONS = [
    "거실 바닥 전체에 물 고임 및 침수 흔적이 확인됩니다.",
    "누수 긴급 보수 및 배수 청소 인건비 결제에 따른 신용카드 승인 영수증입니다. 결제 금액은 150,000원입니다.",
    "지자체 재난안전대책본부에서 발송한 강풍 및 호우경보 발령 안내 긴급재난문자 캡처입니다. 수신 일시는 2026-07-08 14:20입니다.",
]

# A generated description with an invented amount and an invented cause that
# never appeared in any evidence caption above.
HALLUCINATED_DESCRIPTION = (
    "2026-07-08 14:20 호우경보 발령 이후 거실 바닥에 침수 피해가 발생하였습니다. "
    "누수의 원인은 옥상 방수층 노후화로 추정됩니다. "
    "긴급 보수 비용으로 350,000원이 결제되었습니다."
)


def run():
    cleaned, flagged = ground_text(HALLUCINATED_DESCRIPTION, EVIDENCE_CAPTIONS)

    print("=== BEFORE (raw Gemini output) ===")
    print(HALLUCINATED_DESCRIPTION)
    print()
    print("=== AFTER (grounding-checked) ===")
    print(cleaned)
    print()
    print("=== FLAGGED (removed as ungrounded) ===")
    for sentence in flagged:
        print(f"- {sentence}")

    # The invented amount (350,000 vs the real 150,000) must be caught.
    assert "350,000원" not in cleaned, "Invented amount should have been stripped"
    # The invented cause ("옥상 방수층 노후화") has no numeric/date facts, so it can
    # only be caught by the semantic check, which needs GEMINI_API_KEY to run.
    # The real, evidence-backed date should survive.
    assert "2026-07-08 14:20" in cleaned or not flagged, "Grounded facts should be kept"

    print("\nOK: fact-check layer caught the invented amount.")
    if not os.environ.get("GEMINI_API_KEY"):
        print(
            "NOTE: GEMINI_API_KEY not set, so the semantic (embedding) layer was "
            "skipped. The invented cause sentence would also be caught with a key set."
        )


if __name__ == "__main__":
    run()
