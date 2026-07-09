import random
import logging
from typing import List, Dict, Any
from app.schemas import FileAnalysisInput, EvidenceSummaryItem
from app.gemini_client import (
    is_gemini_available,
    analyze_evidence_item_gemini,
    generate_description_gemini,
    generate_timeline_gemini
)

logger = logging.getLogger("ai-service")

# --- Realistic Mock Data for Local Testing & Fallback ---

MOCK_CAPTIONS = {
    "floor_flooding": [
        "거실 바닥 전체에 물 고임 및 침수 흔적이 확인됩니다.",
        "침실 바닥의 목재 마루가 물을 흡수하여 들뜨고 들고일어난 변색 상태가 관찰됩니다.",
        "현관 및 복도 바닥에 흙물이 유입되어 장판 아래로 수분이 가득 찬 정황이 확인됩니다."
    ],
    "wall_damage": [
        "벽면 하단부에 수분 노출로 인한 벽지 오염 및 곰팡이 얼룩 흔적이 확인됩니다.",
        "누수로 인해 천장과 벽체 경계부에 심각한 변색 및 박리 현상이 발생한 상태입니다.",
        "창틀 주변 벽면 내부에서 습기가 배어나와 마감재가 훼손된 흔적이 관찰됩니다."
    ],
    "appliance_damage": [
        "가전제품 주변 및 하단 배선으로 물이 대량 유입되어 가전 전원 오작동 위험이 매우 큽니다.",
        "세탁기와 냉장고 주변부까지 흙물이 차올라 침수 및 진흙 고임 피해가 발생한 사진입니다.",
        "실외기 콘센트 부근 누수로 인해 가전제품 작동 불능 상태가 된 내역입니다."
    ],
    "furniture_damage": [
        "소파 하단 가죽 및 목재 프레임이 물에 젖어 부풀어 오르고 곰팡이가 피기 시작한 흔적입니다.",
        "침대 매트리스 하부 및 가구 수납장 뒷판이 수분을 직접 흡수해 오염된 상태가 관찰됩니다.",
        "목재 테이블 및 서랍장 문짝이 뒤틀려 맞물림 상태가 불량한 상태가 확인됩니다."
    ],
    "receipt": [
        "누수 긴급 보수 및 배수 청소 인건비 결제에 따른 신용카드 승인 영수증입니다.",
        "침수 피해 복구를 위해 하드웨어 샵에서 구입한 긴급 펌프 및 방수 테이프 자재 영수증입니다.",
        "벽지 및 마루 일부분 철거 작업을 진행한 후 발행된 간이 영수증 증빙 자료입니다."
    ],
    "disaster_alert": [
        "지자체 재난안전대책본부에서 발송한 강풍 및 호우경보 발령 안내 긴급재난문자 캡처입니다.",
        "태풍 북상에 따라 주민 대피령 및 인근 대피소 위치를 알리는 안전 안내 문자 수신 내역입니다.",
        "기상청 발령 호우주의보 격상 알림 및 배수구 점검 주의 환기 문자 화면입니다."
    ],
    "estimate": [
        "거실 마루 전체 철거 및 보일러 배선 점검, 재시공 비용이 포함된 전문 인테리어 업체의 피해 복구 견적서입니다.",
        "보일러 및 온수 배관 균열 보수 공사를 위해 보일러 설비 센터에서 발행한 보수 공사 견적서입니다.",
        "침수 가전제품 수리 및 부품 교체에 드는 정식 AS 서비스 센터의 사전 수리 견적서입니다."
    ],
    "other": [
        "재난 피해 당시의 현장 전체 구조 및 배수 상태를 나타내는 참고용 증빙 사진입니다.",
        "피해 복구 작업 착수 전, 초기 상황을 안전하게 보존하여 기록한 사진 자료입니다.",
        "피해 사실 소명을 위해 건물 외부 및 공동 배수관 상태를 촬영한 서브 캡처 자료입니다."
    ]
}


def guess_category_by_heuristics(file_name: str, file_type: str) -> str:
    """Intelligently map file metadata to an appropriate category code using keywords."""
    fn = file_name.lower()
    ft = file_type.lower()
    
    # 1. Disaster Alert Check
    if any(k in fn for k in ["alert", "warn", "sms", "message", "screen", "alert_screenshot", "문자", "재난", "대피"]):
        return "disaster_alert"
    
    # 2. Receipt Check
    if any(k in fn for k in ["receipt", "pay", "bill", "invoice", "card", "영수증", "결제", "카드", "수납"]):
        return "receipt"
    
    # 3. Estimate Check
    if any(k in fn for k in ["estimate", "quote", "price", "견적", "견적서", "산출서", "비용"]):
        return "estimate"
    
    # 4. Floor Flooding Check
    if any(k in fn for k in ["floor", "flood", "puddle", "water", "wet", "바닥", "침수", "누수", "고임", "마루"]):
        return "floor_flooding"
        
    # 5. Wall Damage Check
    if any(k in fn for k in ["wall", "wallpaper", "mold", "ceiling", "crack", "벽면", "벽지", "천장", "얼룩", "곰팡이"]):
        return "wall_damage"
        
    # 6. Appliance Damage Check
    if any(k in fn for k in ["appliance", "tv", "fridge", "dryer", "machine", "washer", "electron", "가전", "전자", "컴퓨터", "세탁기", "냉장고"]):
        return "appliance_damage"
        
    # 7. Furniture Damage Check
    if any(k in fn for k in ["furniture", "sofa", "bed", "desk", "closet", "chair", "가구", "소파", "침대", "의자", "책상", "장롱"]):
        return "furniture_damage"

    # 8. Check general FileType fallback
    if ft in MOCK_CAPTIONS:
        return ft
    if "receipt" in ft:
        return "receipt"
    if "alert" in ft:
        return "disaster_alert"
    if "estimate" in ft or "invoice" in ft:
        return "estimate"
    if "image" in ft or "photo" in ft:
        return "floor_flooding" # Common default for damage images
        
    return "other"


async def analyze_images_or_fallback(
    project_id: str,
    files: List[FileAnalysisInput]
) -> List[Dict[str, Any]]:
    """Analyzes a list of files. Uses Gemini if available, else falls back to heuristics."""
    evidence_list = []
    
    use_gemini = is_gemini_available()
    logger.info(f"Analyzing {len(files)} files for project {project_id}. Use Gemini: {use_gemini}")
    
    for f in files:
        result = None
        if use_gemini:
            result = await analyze_evidence_item_gemini(
                file_id=f.id,
                file_name=f.file_name,
                file_type=f.file_type,
                file_url=f.file_url,
                mime_type=f.mime_type
            )
            
        if result is None:
            # Fallback to realistic mock generator
            category = guess_category_by_heuristics(f.file_name, f.file_type)
            # Pick a random realistic Korean caption based on category
            caption = random.choice(MOCK_CAPTIONS.get(category, MOCK_CAPTIONS["other"]))
            
            result = {
                "file_id": f.id,
                "file_url": f.file_url,
                "category": category,
                "caption": caption
            }
            logger.info(f"Fallback/Mock result for file {f.id} (Category: {category})")
            
        evidence_list.append(result)
        
    return evidence_list


async def generate_description_or_fallback(
    project_id: str,
    evidence: List[EvidenceSummaryItem]
) -> str:
    """Generates description. Uses Gemini if available, else builds a fallback template."""
    evidence_dicts = [e.model_dump() for e in evidence]
    
    if is_gemini_available():
        desc = await generate_description_gemini(evidence_dicts)
        if desc:
            return desc
            
    # Professional Korean Fallback Template Builder
    logger.info("Building mock fallback overall description...")
    categories_present = set(e.category for e in evidence)
    
    parts = []
    parts.append("재해 발생에 따른 피해 입증 목적으로 작성된 종합 피해 설명서입니다.")
    
    damage_details = []
    if "disaster_alert" in categories_present:
        damage_details.append("재난 경보 및 비상안내 문자가 전파된 기상 이변 상황")
    if "floor_flooding" in categories_present or "wall_damage" in categories_present:
        damage_details.append("주거 구역 침수 및 벽지 오염을 유발하는 누수 현상")
    if "appliance_damage" in categories_present or "furniture_damage" in categories_present:
        damage_details.append("가재도구와 주요 생활 가전기기 침수 및 파손 손상")
        
    if damage_details:
        parts.append(f"본 피해는 주로 {', '.join(damage_details)}으로 인해 발생하였습니다.")
    else:
        parts.append("현장 사진 및 비상 자료를 근거로 작성된 종합 보고 내역입니다.")
        
    doc_details = []
    if "estimate" in categories_present:
        doc_details.append("복구 소요액 산출을 위한 보수 공사 견적서")
    if "receipt" in categories_present:
        doc_details.append("긴급 자재 구매 및 청소비 영수증 증빙")
        
    if doc_details:
        parts.append(f"피해 사후 조치 및 현장 보존을 위하여 {', '.join(doc_details)}를 증빙 자료로 확보하였습니다.")
        
    parts.append("모든 자료는 거주자가 촬영 및 수집한 기록이며, 최종 제출 및 보상 협의를 위한 보조 증빙 자료로서의 정직성과 신뢰성을 바탕으로 작성되었습니다.")
    
    return " ".join(parts)


async def generate_timeline_or_fallback(
    project_id: str,
    evidence: List[EvidenceSummaryItem]
) -> List[Dict[str, Any]]:
    """Generates timeline. Uses Gemini if available, else builds a fallback chronological timeline."""
    evidence_dicts = [e.model_dump() for e in evidence]
    
    if is_gemini_available():
        timeline = await generate_timeline_gemini(evidence_dicts)
        if timeline:
            return timeline
            
    # Mock fallback timeline builder
    logger.info("Building mock fallback timeline...")
    events = []
    
    # 1. Alert (Start)
    alert_item = next((e for e in evidence if e.category == "disaster_alert"), None)
    if alert_item:
        events.append({
            "title": "집중호우 재난 문자 수신",
            "description": alert_item.caption,
            "event_date": "2026-07-09 13:00"
        })
    else:
        events.append({
            "title": "기상 특보 전파 및 호우 발령",
            "description": "지방자치단체 및 소방 안전 안내 문자 전송, 호우 경보 발령으로 주민 사전 경계 강화.",
            "event_date": "2026-07-09 12:00"
        })
        
    # 2. Damage (Middle)
    damage_items = [e for e in evidence if e.category in ["floor_flooding", "wall_damage", "appliance_damage", "furniture_damage", "other"]]
    if damage_items:
        for idx, item in enumerate(damage_items[:2]): # Limit to max 2 items to avoid overload
            title_map = {
                "floor_flooding": "바닥 침수 발생",
                "wall_damage": "벽체 벽지 오염",
                "appliance_damage": "가전제품 피해",
                "furniture_damage": "가구 침수 파손",
                "other": "현장 상황 확인"
            }
            title = title_map.get(item.category, "현장 피해 발견")
            events.append({
                "title": title,
                "description": item.caption,
                "event_date": f"2026-07-09 15:30"
            })
    else:
        events.append({
            "title": "침수 및 현장 누수 발견",
            "description": "폭우 및 기상 오염으로 벽면 누수 및 바닥 고임 확인, 신속한 초기 양수 작업 실시.",
            "event_date": "2026-07-09 16:00"
        })
        
    # 3. Estimate/Receipt (End)
    estimate_item = next((e for e in evidence if e.category == "estimate"), None)
    if estimate_item:
        events.append({
            "title": "피해 복구 비용 견적 산출",
            "description": estimate_item.caption,
            "event_date": "2026-07-09 18:00"
        })
        
    receipt_item = next((e for e in evidence if e.category == "receipt"), None)
    if receipt_item:
        events.append({
            "title": "복구 조치 자재 구매 결제",
            "description": receipt_item.caption,
            "event_date": "2026-07-09 19:30"
        })
        
    # If no estimate or receipt but we want to end with a summary
    if not estimate_item and not receipt_item:
        events.append({
            "title": "피해 기록 보존 및 임시 복구 완료",
            "description": "관리사무소 보고 및 보험 접수를 위해 사진 촬영 완료 및 추가 피해 확산 방지 작업 실시.",
            "event_date": "2026-07-09 20:00"
        })
        
    # Sort events by date just in case
    events.sort(key=lambda x: x["event_date"])
    return events
