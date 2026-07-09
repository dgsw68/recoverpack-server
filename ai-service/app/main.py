import os
import logging
from fastapi import FastAPI, HTTPException, status
from fastapi.middleware.cors import CORSMiddleware
from dotenv import load_dotenv

# Load local environment variables (if any)
load_dotenv()

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("ai-service")

from app.schemas import (
    AnalyzeImageRequest,
    AnalyzeImageResponse,
    GenerateDescriptionRequest,
    GenerateDescriptionResponse,
    GenerateTimelineRequest,
    GenerateTimelineResponse,
    EvidenceAnalysisItem,
    TimelineEventItem
)
from app.analyzer import (
    analyze_images_or_fallback,
    generate_description_or_fallback,
    generate_timeline_or_fallback
)
from app.gemini_client import is_gemini_available

app = FastAPI(
    title="RecoverPack AI Service",
    description="FastAPI service for disaster damage analysis, classification, and text summaries.",
    version="1.0.0"
)

# Enable CORS for local testing if needed
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)


@app.get("/health", status_code=status.HTTP_200_OK)
async def health_check():
    """Health check endpoint confirming service is alive and status of Gemini key."""
    gemini_ready = is_gemini_available()
    return {
        "status": "healthy",
        "service": "ai-service",
        "gemini_api_key_configured": gemini_ready,
        "mode": "gemini-active" if gemini_ready else "mock-fallback-active"
    }


@app.post(
    "/internal/analyze-image",
    response_model=AnalyzeImageResponse,
    status_code=status.HTTP_200_OK
)
async def analyze_image(payload: AnalyzeImageRequest):
    """
    Analyzes multiple file uploads.
    Under the hood, it classifies each file (e.g. wall damage, receipts) and writes natural,
    objective Korean descriptions/captions for documentation.
    """
    logger.info(f"Received image analysis request for project_id: {payload.project_id}")
    if not payload.files:
        return AnalyzeImageResponse(evidence=[])

    try:
        raw_evidence = await analyze_images_or_fallback(payload.project_id, payload.files)
        
        # Convert dictionary list to pydantic model schema
        evidence_items = [
            EvidenceAnalysisItem(
                file_id=item["file_id"],
                file_url=item["file_url"],
                category=item["category"],
                caption=item["caption"]
            )
            for item in raw_evidence
        ]
        return AnalyzeImageResponse(evidence=evidence_items)
    except Exception as e:
        logger.error(f"Error during image analysis: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Internal Image Analysis Error: {str(e)}"
        )


@app.post(
    "/internal/generate-description",
    response_model=GenerateDescriptionResponse,
    status_code=status.HTTP_200_OK
)
async def generate_description(payload: GenerateDescriptionRequest):
    """
    Synthesizes multiple evidence pieces into a cohesive, professional Korean summary
    of the overall disaster event and subsequent documentation efforts.
    """
    logger.info(f"Received description generation request for project_id: {payload.project_id}")
    try:
        desc = await generate_description_or_fallback(payload.project_id, payload.evidence)
        return GenerateDescriptionResponse(description=desc)
    except Exception as e:
        logger.error(f"Error generating description: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Internal Description Generation Error: {str(e)}"
        )


@app.post(
    "/internal/generate-timeline",
    response_model=GenerateTimelineResponse,
    status_code=status.HTTP_200_OK
)
async def generate_timeline(payload: GenerateTimelineRequest):
    """
    Assembles evidence into a chronological series of events representing the timeline of
    disaster onset, damage capture, repair estimations, and bill settlements.
    """
    logger.info(f"Received timeline generation request for project_id: {payload.project_id}")
    try:
        raw_events = await generate_timeline_or_fallback(payload.project_id, payload.evidence)
        
        events = [
            TimelineEventItem(
                title=item["title"],
                description=item["description"],
                event_date=item["event_date"]
            )
            for item in raw_events
        ]
        return GenerateTimelineResponse(timeline=events)
    except Exception as e:
        logger.error(f"Error generating timeline: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Internal Timeline Generation Error: {str(e)}"
        )
