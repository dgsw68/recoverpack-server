import os
import json
import base64
import logging
import requests
from io import BytesIO
from typing import Optional, List, Dict, Any
from PIL import Image

import google.generativeai as genai
from app.prompts import IMAGE_ANALYSIS_SYSTEM, DESCRIPTION_GENERATION_SYSTEM, TIMELINE_GENERATION_SYSTEM

logger = logging.getLogger("ai-service")

# Retrieve Gemini API Key from environment
GEMINI_API_KEY = os.environ.get("GEMINI_API_KEY")

if GEMINI_API_KEY:
    genai.configure(api_key=GEMINI_API_KEY)
    logger.info("Gemini API configured successfully.")
else:
    logger.warning("GEMINI_API_KEY not found in environment. Running in Mock Mode.")


def is_gemini_available() -> bool:
    """Returns True if Gemini API is configured and ready."""
    return bool(GEMINI_API_KEY)


def fetch_image(url: str) -> Optional[Image.Image]:
    """Helper to download an image from a URL and return a PIL Image."""
    try:
        logger.info(f"Downloading image from: {url}")
        response = requests.get(url, timeout=10)
        response.raise_for_status()
        img = Image.open(BytesIO(response.content))
        # Convert to RGB if needed to avoid format compatibility issues
        if img.mode not in ('RGB', 'RGBA'):
            img = img.convert('RGB')
        return img
    except Exception as e:
        logger.error(f"Failed to fetch image from URL {url}: {e}")
        return None


async def analyze_evidence_item_gemini(
    file_id: str,
    file_name: str,
    file_type: str,
    file_url: str,
    mime_type: str,
    content_base64: Optional[str] = None
) -> Optional[Dict[str, Any]]:
    """
    Analyzes an evidence file using Gemini 1.5 Flash.
    Downloads the image if it's an image MIME type.
    """
    if not is_gemini_available():
        return None

    try:
        model = genai.GenerativeModel(
            model_name="gemini-1.5-flash",
            system_instruction=IMAGE_ANALYSIS_SYSTEM
        )

        prompt = f"파일명: {file_name}\n파일타입: {file_type}\nMIME타입: {mime_type}\n파일URL: {file_url}\n\n이 파일에 대한 피해 카테고리를 분류하고 상세 캡션을 작성해 주세요."

        # If it's an image, fetch and include the image data
        image_data = None
        if mime_type.startswith("image/"):
            if content_base64:
                image_data = Image.open(BytesIO(base64.b64decode(content_base64, validate=True)))
                if image_data.mode not in ('RGB', 'RGBA'):
                    image_data = image_data.convert('RGB')
            else:
                image_data = fetch_image(file_url)

        contents = []
        if image_data:
            contents.append(image_data)
        contents.append(prompt)

        logger.info(f"Calling Gemini to analyze file {file_id} ({file_name})...")
        response = model.generate_content(
            contents,
            generation_config={"response_mime_type": "application/json"}
        )

        if response and response.text:
            result = json.loads(response.text.strip())
            return {
                "file_id": file_id,
                "file_url": file_url,
                "category": result.get("category", "other"),
                "caption": result.get("caption", "분석된 설명이 없습니다.")
            }
    except Exception as e:
        logger.error(f"Gemini image analysis error for file {file_id}: {e}", exc_info=True)
    
    return None


async def generate_description_gemini(
    evidence_items: List[Dict[str, Any]]
) -> Optional[str]:
    """Generates a final overall damage description from a list of evidence items."""
    if not is_gemini_available():
        return None

    try:
        model = genai.GenerativeModel(
            model_name="gemini-1.5-flash",
            system_instruction=DESCRIPTION_GENERATION_SYSTEM
        )

        evidence_summary_str = ""
        for i, item in enumerate(evidence_items, 1):
            evidence_summary_str += f"{i}. [카테고리: {item['category']}] - {item['caption']}\n"

        prompt = f"다음은 분석된 증빙 내역들의 리스트입니다:\n\n{evidence_summary_str}\n위 내역들을 종합하여 격식 있는 종합 피해 보고서 서술글을 한국어로 작성해 주세요."

        logger.info("Calling Gemini to generate overall damage description...")
        response = model.generate_content(prompt)

        if response and response.text:
            return response.text.strip()
    except Exception as e:
        logger.error(f"Gemini description generation error: {e}", exc_info=True)

    return None


async def generate_timeline_gemini(
    evidence_items: List[Dict[str, Any]]
) -> Optional[List[Dict[str, Any]]]:
    """Generates a list of chronological timeline events from evidence items."""
    if not is_gemini_available():
        return None

    try:
        model = genai.GenerativeModel(
            model_name="gemini-1.5-flash",
            system_instruction=TIMELINE_GENERATION_SYSTEM
        )

        evidence_summary_str = ""
        for i, item in enumerate(evidence_items, 1):
            evidence_summary_str += f"{i}. [카테고리: {item['category']}] - {item['caption']}\n"

        prompt = f"다음 증빙 자료들을 시간적 흐름에 맞춰 정렬하고, 주요 타임라인 이벤트를 JSON 배열 형태로 출력해 주세요:\n\n{evidence_summary_str}"

        logger.info("Calling Gemini to generate chronological timeline...")
        response = model.generate_content(
            prompt,
            generation_config={"response_mime_type": "application/json"}
        )

        if response and response.text:
            return json.loads(response.text.strip())
    except Exception as e:
        logger.error(f"Gemini timeline generation error: {e}", exc_info=True)

    return None
