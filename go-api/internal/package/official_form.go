package pckg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-pdf/fpdf"
	"recoverpack-server/go-api/internal/models"
)

// officialDamageCategories are the six 피해 구분 values used on the official
// 자연재난 피해신고서. Final determination is always made by the receiving
// official after a site visit — this mapping is only a reference suggestion.
var officialCategoryKeywords = []struct {
	keyword  string
	category string
}{
	{"매몰", "매몰"},
	{"침수", "침수"},
	{"누수", "침수"},
	{"유실", "유실"},
	{"유출", "유실"},
	{"붕괴", "전파"},
	{"전파", "전파"},
	{"반파", "반파"},
	{"파손", "소파"},
	{"오염", "소파"},
	{"화재", "소파"},
	{"손상", "소파"},
}

// mapToOfficialDamageCategory suggests one of the six official 피해 구분 values
// for an AI-assigned category label. It never claims certainty: callers must
// present the result as a reference, not a final determination.
func mapToOfficialDamageCategory(aiCategory string) string {
	for _, entry := range officialCategoryKeywords {
		if strings.Contains(aiCategory, entry.keyword) {
			return entry.category
		}
	}
	return "담당자 확인 필요"
}

func pdfCheckbox(checked bool) string {
	if checked {
		return "[V]"
	}
	return "[ ]"
}

func pdfSectionHeader(pdf *fpdf.Fpdf, text string) {
	pdf.Ln(3)
	pdf.SetFillColor(20, 82, 99)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont(pdfFontFamily, "", 10.5)
	pdf.CellFormat(180, 8, text, "", 1, "L", true, 0, "")
}

func pdfBlankNote(pdf *fpdf.Fpdf, text string) {
	pdf.SetTextColor(148, 163, 184)
	pdf.SetFont(pdfFontFamily, "", 7.5)
	pdf.MultiCell(180, 4.5, text, "", "L", false)
}

// buildOfficialFormPDF renders a draft of the official 자연재난 피해신고서
// (행정안전부 별지 제1호서식), prefilled with what RecoverPack already knows
// and leaving official-only fields (피해 구분 확정, 피해 물량 확정, 담당 공무원
// 확인 사항, 서명) blank for the user or receiving official to complete by hand.
func buildOfficialFormPDF(project *models.Project, evidence []models.Evidence) ([]byte, error) {
	pdf, err := newPDF("자연재난 피해신고서 (작성 보조본)")
	if err != nil {
		return nil, err
	}
	pdf.AddPage()
	pdfTitle(pdf, "자연재난 피해신고서 (작성 보조본)", "행정안전부 별지 제1호서식 참고 작성본 - 공식 서식이 아니며, 접수 전 담당 공무원 확인이 필요합니다")

	pdfSectionHeader(pdf, "1. 피해자 정보")
	pdfField(pdf, "성명(대표자)", project.ReporterName)
	pdfField(pdf, "주소(사업장)", project.ReporterAddress)
	pdfField(pdf, "연락처", project.ReporterPhone)
	pdfField(pdf, "주거 형태", project.ResidenceType)
	pdfBlankNote(pdf, "주민등록번호, 가족 수, 재난지원금 지급계좌 등은 개인정보 보호를 위해 본 자료에 포함하지 않았습니다. 제출 시 신고인이 직접 기재해야 합니다.")

	pdfSectionHeader(pdf, "2. 피해 내용 (AI 분류 기반 참고 목록)")
	buildDamageItemsTable(pdf, project, evidence)
	pdfBlankNote(pdf, "총면적, 면허·허가·등록번호, 피해 물량(신고/확정), 피해 구분 확정치는 담당 공무원의 현지 확인 후 기재되는 항목으로, 본 자료에는 포함되지 않습니다. '피해 구분(AI 참고)'는 최종 판정이 아닙니다.")

	pdfSectionHeader(pdf, "3. 간접 지원 확인")
	support := project.IndirectSupport
	indirectRows := []struct {
		label   string
		checked bool
	}{
		{"도시가스 사용", support.GasUser},
		{"자동차 소유", support.VehicleOwner},
		{"공공주택 임대 신청 희망", support.PublicHousingRequest},
		{"위기가족 긴급지원 희망", support.FamilyCrisisSupport},
		{"건강보험료 체납", support.HealthInsuranceArrears},
		{"과태료 징수 유예 희망", support.FineDeferralRequest},
		{"재해손실 공제 신청 희망", support.DisasterLossDeduction},
		{"풍수해보험 가입 의사", support.WindFloodInsuranceOptIn},
	}
	for _, row := range indirectRows {
		pdf.SetFillColor(241, 245, 249)
		pdf.SetTextColor(15, 23, 42)
		pdf.SetFont(pdfFontFamily, "", 9)
		pdf.CellFormat(10, 7, pdfCheckbox(row.checked), "B", 0, "C", true, 0, "")
		pdf.CellFormat(170, 7, row.label, "B", 1, "L", false, 0, "")
	}
	pdfBlankNote(pdf, "각 항목의 실제 지원 여부는 간접지원 실시기관별 확인을 거쳐 결정됩니다. 위 체크는 신고인이 등록한 참고 정보입니다.")

	pdfSectionHeader(pdf, "4. 제출 시 함께 준비할 서류 (담당 공무원 확인 사항)")
	for _, doc := range []string{"주민등록표 등본", "소득금액 증명", "가족관계증명서"} {
		pdf.SetFillColor(241, 245, 249)
		pdf.SetTextColor(15, 23, 42)
		pdf.SetFont(pdfFontFamily, "", 9)
		pdf.CellFormat(10, 7, "[ ]", "B", 0, "C", true, 0, "")
		pdf.CellFormat(170, 7, doc+" (신고인이 별도로 발급받아 제출)", "B", 1, "L", false, 0, "")
	}

	pdf.Ln(6)
	pdf.SetFillColor(255, 247, 237)
	pdf.SetTextColor(124, 45, 18)
	pdf.SetFont(pdfFontFamily, "", 8)
	pdf.MultiCell(180, 5, "주의: 이 문서는 행정안전부 별지 제1호서식(자연재난 피해신고서)의 작성을 돕기 위한 참고 자료이며, 공식 서식을 대체하지 않습니다. 신고인 서명란, 행정정보 공동이용 동의, 개인정보 수집·이용 동의, 개인정보 제3자 제공·활용 동의는 접수 기관에서 원본 서식으로 직접 서명해야 합니다. AI가 제안한 피해 구분과 물량은 담당 공무원의 현지 확인 결과로 대체됩니다.", "", "L", true)

	return pdfBytes(pdf)
}

func buildDamageItemsTable(pdf *fpdf.Fpdf, project *models.Project, evidence []models.Evidence) {
	type item struct {
		category string
		official string
		count    int
	}
	counts := make(map[string]int)
	var order []string
	for _, ev := range evidence {
		label := strings.TrimSpace(ev.Category)
		if label == "" {
			continue
		}
		if _, seen := counts[label]; !seen {
			order = append(order, label)
		}
		counts[label]++
	}
	sort.Strings(order)

	items := make([]item, 0, len(order))
	for _, label := range order {
		items = append(items, item{category: label, official: mapToOfficialDamageCategory(label), count: counts[label]})
	}

	pdf.SetFillColor(241, 245, 249)
	pdf.SetTextColor(51, 65, 85)
	pdf.SetFont(pdfFontFamily, "", 8.5)
	pdf.CellFormat(10, 7, "순번", "B", 0, "C", true, 0, "")
	pdf.CellFormat(65, 7, "피해시설명(AI 분류)", "B", 0, "L", true, 0, "")
	pdf.CellFormat(35, 7, "피해 원인", "B", 0, "L", true, 0, "")
	pdf.CellFormat(40, 7, "피해 구분(AI 참고)", "B", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, "관련 사진 수", "B", 1, "C", true, 0, "")

	if len(items) == 0 {
		pdf.SetTextColor(100, 116, 139)
		pdf.SetFont(pdfFontFamily, "", 8.5)
		pdf.CellFormat(180, 7, "등록된 AI 분류 증빙이 없습니다.", "B", 1, "L", false, 0, "")
		return
	}

	for i, it := range items {
		pdf.SetTextColor(15, 23, 42)
		pdf.SetFont(pdfFontFamily, "", 8.5)
		pdf.CellFormat(10, 7, fmt.Sprint(i+1), "B", 0, "C", false, 0, "")
		pdf.CellFormat(65, 7, it.category, "B", 0, "L", false, 0, "")
		pdf.CellFormat(35, 7, project.DamageType, "B", 0, "L", false, 0, "")
		pdf.CellFormat(40, 7, it.official, "B", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%d건", it.count), "B", 1, "C", false, 0, "")
	}
}
