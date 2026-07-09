from pydantic import BaseModel, Field
from typing import List, Optional

# --- Analyze Image ---

class FileAnalysisInput(BaseModel):
    id: str = Field(..., description="Unique ID of the file")
    file_name: str = Field(..., description="Name of the file")
    file_type: str = Field(..., description="Type of the file (image, receipt, estimate, alert, etc.)")
    file_url: str = Field(..., description="Public download URL of the file")
    mime_type: str = Field(..., description="MIME type of the file")
    content_base64: Optional[str] = Field(None, description="Base64-encoded uploaded image bytes")

class AnalyzeImageRequest(BaseModel):
    project_id: str = Field(..., description="ID of the associated damage project")
    files: List[FileAnalysisInput] = Field(..., description="List of files to analyze")

class EvidenceAnalysisItem(BaseModel):
    file_id: str = Field(..., description="Referenced file ID")
    file_url: str = Field(..., description="Referenced file URL")
    category: str = Field(..., description="Classified category of the damage or document")
    caption: str = Field(..., description="Korean caption summarizing the evidence")

class AnalyzeImageResponse(BaseModel):
    evidence: List[EvidenceAnalysisItem]


# --- Generate Description ---

class EvidenceSummaryItem(BaseModel):
    file_id: str = Field(..., description="Referenced file ID")
    category: str = Field(..., description="Category of the evidence")
    caption: str = Field(..., description="Korean caption/description of the evidence")

class GenerateDescriptionRequest(BaseModel):
    project_id: str = Field(..., description="ID of the associated damage project")
    evidence: List[EvidenceSummaryItem] = Field(..., description="List of analyzed evidence items")

class GenerateDescriptionResponse(BaseModel):
    description: str = Field(..., description="Generated final damage description text in Korean")


# --- Generate Timeline ---

class GenerateTimelineRequest(BaseModel):
    project_id: str = Field(..., description="ID of the associated damage project")
    evidence: List[EvidenceSummaryItem] = Field(..., description="List of analyzed evidence items")

class TimelineEventItem(BaseModel):
    title: str = Field(..., description="Title of the timeline event")
    description: str = Field(..., description="Detailed description of the event")
    event_date: str = Field(..., description="Event date/time (e.g. YYYY-MM-DD HH:MM)")

class GenerateTimelineResponse(BaseModel):
    timeline: List[TimelineEventItem]
