import asyncio
import base64
import json
import logging
import os
from typing import Any, Dict, List, Optional

from google import genai
from google.genai import types
from pydantic import BaseModel, Field

from app.harness import run_with_harness
from app.prompts import (
    DESCRIPTION_GENERATION_SYSTEM,
    IMAGE_ANALYSIS_SYSTEM,
    TIMELINE_GENERATION_SYSTEM,
)

logger = logging.getLogger("ai-service")


class EvidenceResult(BaseModel):
    category: str = Field(description="Allowed RecoverPack evidence category code")
    caption: str = Field(description="Objective Korean caption based only on visible evidence")


class TimelineResult(BaseModel):
    title: str
    description: str
    event_date: str = Field(description="Evidence-backed date/time or an empty string")


GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY", "").strip()
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-3.5-flash").strip()
GEMINI_EMBEDDING_MODEL = os.environ.get("GEMINI_EMBEDDING_MODEL", "text-embedding-004").strip()
client = genai.Client(api_key=GEMINI_API_KEY) if GEMINI_API_KEY else None

if client:
    logger.info("Gemini API configured with model %s.", GEMINI_MODEL)
else:
    logger.warning("GEMINI_API_KEY not found. Running in explicitly labeled mock mode.")


def is_gemini_available() -> bool:
    return client is not None


def embed_texts(texts: List[str]) -> Optional[List[List[float]]]:
    """Embeds a list of texts for semantic grounding checks. Returns None if unavailable."""
    if not client or not texts:
        return None
    try:
        embeddings: List[List[float]] = []
        for text in texts:
            response = client.models.embed_content(model=GEMINI_EMBEDDING_MODEL, contents=text)
            embeddings.append(response.embeddings[0].values)
        return embeddings
    except Exception as error:
        logger.error("Gemini embedding failed: %s", error)
        return None


async def analyze_evidence_item_gemini(
    file_id: str,
    file_name: str,
    file_type: str,
    file_url: str,
    mime_type: str,
    content_base64: Optional[str] = None,
) -> Optional[Dict[str, Any]]:
    if not client:
        return None

    prompt = (
        f"파일명: {file_name}\n파일타입: {file_type}\n"
        f"MIME타입: {mime_type}\n\n"
        "제공된 자료에서 직접 확인 가능한 내용만 사용해 피해 카테고리와 캡션을 작성하세요. "
        "확인할 수 없는 손상, 금액, 일시, 원인을 추측하지 마세요."
    )
    contents: List[Any] = [prompt]
    if content_base64 and mime_type.startswith("image/"):
        try:
            image_bytes = base64.b64decode(content_base64, validate=True)
            contents.insert(0, types.Part.from_bytes(data=image_bytes, mime_type=mime_type))
        except (ValueError, TypeError) as error:
            logger.warning("Invalid embedded image for %s: %s", file_id, error)

    def _call():
        return client.models.generate_content(
            model=GEMINI_MODEL,
            contents=contents,
            config=types.GenerateContentConfig(
                system_instruction=IMAGE_ANALYSIS_SYSTEM,
                response_mime_type="application/json",
                response_schema=EvidenceResult,
                temperature=0.1,
            ),
        )

    response = await run_with_harness(
        f"analyze_evidence_item:{file_id}",
        lambda: asyncio.to_thread(_call),
    )
    if response is None:
        return None

    try:
        result = (
            response.parsed.model_dump()
            if isinstance(response.parsed, EvidenceResult)
            else json.loads(response.text)
        )
        return {
            "file_id": file_id,
            "file_url": file_url,
            "category": result.get("category", "other"),
            "caption": result.get("caption", "자료에서 확인 가능한 설명이 없습니다."),
        }
    except Exception as error:
        logger.error("Gemini image analysis parsing failed for %s: %s", file_id, error)
        return None


async def generate_description_gemini(
    evidence_items: List[Dict[str, Any]],
) -> Optional[str]:
    if not client:
        return None
    evidence_text = "\n".join(
        f"{index}. [{item['category']}] {item['caption']}"
        for index, item in enumerate(evidence_items, 1)
    )
    def _call():
        return client.models.generate_content(
            model=GEMINI_MODEL,
            contents=f"다음 증빙 내역만 근거로 피해 설명문을 작성하세요.\n\n{evidence_text}",
            config=types.GenerateContentConfig(
                system_instruction=DESCRIPTION_GENERATION_SYSTEM,
                temperature=0.1,
            ),
        )

    response = await run_with_harness(
        "generate_description",
        lambda: asyncio.to_thread(_call),
    )
    if response is None:
        return None
    return response.text.strip() if response.text else None


async def generate_timeline_gemini(
    evidence_items: List[Dict[str, Any]],
) -> Optional[List[Dict[str, Any]]]:
    if not client:
        return None
    evidence_text = "\n".join(
        f"{index}. [{item['category']}] {item['caption']}"
        for index, item in enumerate(evidence_items, 1)
    )
    def _call():
        return client.models.generate_content(
            model=GEMINI_MODEL,
            contents=(
                "다음 증빙에 명시된 사건과 일시만 타임라인 JSON 배열로 정리하세요. "
                "명시되지 않은 날짜나 시각은 만들지 말고 event_date를 빈 문자열로 두세요.\n\n"
                f"{evidence_text}"
            ),
            config=types.GenerateContentConfig(
                system_instruction=TIMELINE_GENERATION_SYSTEM,
                response_mime_type="application/json",
                response_schema=list[TimelineResult],
                temperature=0,
            ),
        )

    response = await run_with_harness(
        "generate_timeline",
        lambda: asyncio.to_thread(_call),
    )
    if response is None:
        return None

    try:
        parsed = (
            [item.model_dump() for item in response.parsed]
            if isinstance(response.parsed, list)
            else json.loads(response.text)
        )
        return parsed if isinstance(parsed, list) else None
    except Exception as error:
        logger.error("Gemini timeline generation parsing failed: %s", error)
        return None
