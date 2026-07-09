"""
Post-generation hallucination guard.

Prompts already tell Gemini not to invent facts, but nothing verified that it
actually complied. This module checks generated text against the evidence it
was supposed to be based on, using two independent signals:

1. Fact check (regex): numbers/amounts/dates that don't appear anywhere in
   the source evidence captions are treated as invented and dropped.
2. Semantic check (embeddings): sentences whose meaning has no close match in
   any evidence caption are treated as unsupported and dropped. This catches
   qualitative hallucinations (invented causes, invented descriptions) that a
   numeric regex can't see.

Both checks are best-effort: if Gemini/embeddings are unavailable, the
semantic check is skipped and only the fact check runs.
"""
import re
import logging
from typing import List, Tuple

from app.gemini_client import embed_texts, is_gemini_available

logger = logging.getLogger("ai-service")

# Below this cosine similarity to every evidence caption, a sentence is
# considered semantically unsupported. Tuned conservatively to avoid
# stripping legitimate connective/summary sentences.
SIMILARITY_THRESHOLD = 0.45

# Only run the semantic check on sentences long enough to carry a real claim;
# short scaffolding sentences ("본 보고서는 ~ 입니다.") aren't hallucination risks.
MIN_SEMANTIC_CHECK_LENGTH = 20

NUMBER_PATTERN = re.compile(r"\d[\d,]*(?:\s*(?:원|만원|천원|%))?")
DATE_PATTERN = re.compile(r"\d{4}[-./]\d{1,2}[-./]\d{1,2}|\d{1,2}:\d{2}")


def _split_sentences(text: str) -> List[str]:
    sentences = re.split(r"(?<=[.!?])\s+", text.strip())
    return [s.strip() for s in sentences if s.strip()]


def _extract_facts(text: str) -> set:
    return set(NUMBER_PATTERN.findall(text)) | set(DATE_PATTERN.findall(text))


def _cosine(a: List[float], b: List[float]) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    norm_a = sum(x * x for x in a) ** 0.5
    norm_b = sum(y * y for y in b) ** 0.5
    if norm_a == 0 or norm_b == 0:
        return 0.0
    return dot / (norm_a * norm_b)


def ground_text(generated_text: str, evidence_captions: List[str]) -> Tuple[str, List[str]]:
    """
    Verifies `generated_text` against `evidence_captions` and strips any
    sentence that isn't supported by them.

    Returns (cleaned_text, flagged_sentences). If every sentence gets
    flagged (likely a false positive from the heuristics), the original text
    is returned unchanged rather than emptied out.
    """
    sentences = _split_sentences(generated_text)
    if not sentences or not evidence_captions:
        return generated_text, []

    evidence_facts: set = set()
    for caption in evidence_captions:
        evidence_facts |= _extract_facts(caption)

    use_semantic = is_gemini_available()
    sentence_embeddings = embed_texts(sentences) if use_semantic else None
    evidence_embeddings = embed_texts(evidence_captions) if use_semantic else None
    use_semantic = bool(sentence_embeddings and evidence_embeddings)

    flagged: List[str] = []
    kept: List[str] = []

    for idx, sentence in enumerate(sentences):
        unsupported_facts = _extract_facts(sentence) - evidence_facts
        if unsupported_facts:
            logger.warning(
                "Grounding: dropped fact-inconsistent sentence %r (unsupported: %s)",
                sentence, unsupported_facts,
            )
            flagged.append(sentence)
            continue

        if use_semantic and len(sentence) >= MIN_SEMANTIC_CHECK_LENGTH:
            best_sim = max(_cosine(sentence_embeddings[idx], e) for e in evidence_embeddings)
            if best_sim < SIMILARITY_THRESHOLD:
                logger.warning(
                    "Grounding: dropped semantically-ungrounded sentence %r (max_sim=%.2f)",
                    sentence, best_sim,
                )
                flagged.append(sentence)
                continue

        kept.append(sentence)

    if not kept:
        # Every sentence got flagged - almost certainly a false positive from
        # the heuristics rather than a fully hallucinated response. Prefer
        # returning the original text over an empty description.
        logger.warning("Grounding: all sentences flagged, keeping original text unfiltered.")
        return generated_text, flagged

    return " ".join(kept), flagged
