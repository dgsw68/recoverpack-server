package pckg

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"recoverpack-server/go-api/internal/models"
)

const pdfFontFamily = "RecoverPackKorean"

func pdfFontPath() (string, error) {
	candidates := []string{
		os.Getenv("RECOVERPACK_PDF_FONT"),
		"/usr/share/fonts/noto/NotoSansCJK-Regular.ttf",
		"/usr/share/fonts/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/System/Library/Fonts/Supplemental/AppleGothic.ttf",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("Korean PDF font not found; set RECOVERPACK_PDF_FONT")
}

func newPDF(title string) (*fpdf.Fpdf, error) {
	fontPath, err := pdfFontPath()
	if err != nil {
		return nil, err
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle(title, true)
	pdf.SetAuthor("RecoverPack", true)
	pdf.SetMargins(15, 14, 15)
	pdf.SetAutoPageBreak(true, 15)
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("read PDF font: %w", err)
	}
	pdf.AddUTF8FontFromBytes(pdfFontFamily, "", fontData)
	if pdf.Error() != nil {
		return nil, fmt.Errorf("load PDF font: %w", pdf.Error())
	}
	pdf.SetFont(pdfFontFamily, "", 10)
	return pdf, nil
}

func pdfBytes(pdf *fpdf.Fpdf) ([]byte, error) {
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return nil, fmt.Errorf("render PDF: %w", err)
	}
	return output.Bytes(), nil
}

func pdfTitle(pdf *fpdf.Fpdf, title, subtitle string) {
	pdf.SetFillColor(20, 82, 99)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont(pdfFontFamily, "", 18)
	pdf.CellFormat(180, 13, title, "", 1, "L", true, 0, "")
	pdf.SetFillColor(231, 245, 247)
	pdf.SetTextColor(51, 65, 85)
	pdf.SetFont(pdfFontFamily, "", 8.5)
	pdf.CellFormat(180, 8, subtitle, "", 1, "L", true, 0, "")
	pdf.Ln(4)
}

func pdfField(pdf *fpdf.Fpdf, label, value string) {
	if strings.TrimSpace(value) == "" {
		value = "확인되지 않음"
	}
	pdf.SetFillColor(241, 245, 249)
	pdf.SetTextColor(51, 65, 85)
	pdf.SetFont(pdfFontFamily, "", 9)
	pdf.CellFormat(38, 8, label, "B", 0, "L", true, 0, "")
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(142, 8, value, "B", 1, "L", false, 0, "")
}

func buildSummaryPDF(project *models.Project, files []models.ProjectFile, evidence []models.Evidence, timeline []models.TimelineEvent) ([]byte, error) {
	pdf, err := newPDF("접수용 1페이지 요약표")
	if err != nil {
		return nil, err
	}
	pdf.AddPage()
	pdfTitle(pdf, "재난 피해 증빙 요약표", "접수 담당자의 빠른 확인을 위한 제출 보조 자료")
	pdfField(pdf, "신고인", project.ReporterName)
	pdfField(pdf, "연락처", project.ReporterPhone)
	pdfField(pdf, "신고인 주소", project.ReporterAddress)
	pdfField(pdf, "프로젝트명", project.Title)
	pdfField(pdf, "재난 유형", project.DamageType)
	pdfField(pdf, "발생 일시", project.OccurredAt)
	pdfField(pdf, "피해 위치", project.Location)
	pdfField(pdf, "첨부 자료", fmt.Sprintf("원본 %d건 / 분류 증빙 %d건 / 타임라인 %d건", len(files), len(evidence), len(timeline)))
	pdf.Ln(5)
	pdf.SetTextColor(20, 82, 99)
	pdf.SetFont(pdfFontFamily, "", 12)
	pdf.CellFormat(180, 8, "피해 설명", "", 1, "L", false, 0, "")
	pdf.SetTextColor(30, 41, 59)
	pdf.SetFont(pdfFontFamily, "", 9.5)
	description := strings.TrimSpace(project.Description)
	if description == "" {
		description = "등록된 피해 설명이 없습니다."
	}
	pdf.MultiCell(180, 6, description, "1", "L", false)
	pdf.Ln(5)
	pdf.SetFillColor(255, 247, 237)
	pdf.SetTextColor(124, 45, 18)
	pdf.SetFont(pdfFontFamily, "", 8)
	pdf.MultiCell(180, 5, "주의: 본 문서는 제출 편의를 위한 보조 자료이며 보상 가능 여부, 법적 책임 또는 피해 원인을 판단하지 않습니다. 내용과 날짜는 제출 전에 반드시 확인하십시오.", "", "L", true)
	return pdfBytes(pdf)
}

func buildTimelinePDF(project *models.Project, events []models.TimelineEvent) ([]byte, error) {
	pdf, err := newPDF("재난문자 및 피해 타임라인")
	if err != nil {
		return nil, err
	}
	pdf.AddPage()
	pdfTitle(pdf, "재난문자 · 피해 타임라인", project.Title+" - 증빙에 기록된 내용만 정리")
	sorted := append([]models.TimelineEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].EventDate == "" {
			return false
		}
		if sorted[j].EventDate == "" {
			return true
		}
		return sorted[i].EventDate < sorted[j].EventDate
	})
	if len(sorted) == 0 {
		pdf.SetTextColor(71, 85, 105)
		pdf.MultiCell(180, 7, "등록된 타임라인 항목이 없습니다.", "1", "L", false)
	} else {
		for i, event := range sorted {
			if pdf.GetY() > 255 {
				pdf.AddPage()
				pdfTitle(pdf, "재난문자 · 피해 타임라인", "계속")
			}
			date := strings.TrimSpace(event.EventDate)
			if date == "" {
				date = "일시 확인되지 않음"
			}
			pdf.SetFillColor(20, 82, 99)
			pdf.SetTextColor(255, 255, 255)
			pdf.SetFont(pdfFontFamily, "", 9)
			pdf.CellFormat(180, 7, fmt.Sprintf("%02d  %s", i+1, date), "", 1, "L", true, 0, "")
			pdf.SetFillColor(241, 245, 249)
			pdf.SetTextColor(15, 23, 42)
			pdf.SetFont(pdfFontFamily, "", 10)
			pdf.CellFormat(180, 7, event.Title, "", 1, "L", true, 0, "")
			pdf.SetFont(pdfFontFamily, "", 8.5)
			pdf.MultiCell(180, 5.5, event.Description, "B", "L", false)
			pdf.Ln(4)
		}
	}
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont(pdfFontFamily, "", 7.5)
	pdf.MultiCell(180, 5, "일시가 비어 있는 항목은 원본 증빙에서 날짜나 시간을 확인할 수 없음을 의미합니다. 임의 일시는 추가하지 않았습니다.", "", "L", false)
	return pdfBytes(pdf)
}

type xlsxCell struct {
	value string
	style int
}

func buildAttachmentIndexXLSX(files []models.ProjectFile, evidence []models.Evidence) ([]byte, error) {
	byFileID := make(map[string]models.Evidence, len(evidence))
	for _, item := range evidence {
		byFileID[item.FileID] = item
	}
	rows := [][]xlsxCell{{
		{value: "순번", style: 2}, {value: "파일명", style: 2}, {value: "파일유형", style: 2},
		{value: "MIME", style: 2}, {value: "AI 분류", style: 2}, {value: "객관적 설명", style: 2},
		{value: "등록시각", style: 2},
	}}
	for i, file := range files {
		item := byFileID[file.ID]
		createdAt := ""
		if !file.CreatedAt.IsZero() {
			createdAt = file.CreatedAt.Format(time.RFC3339)
		}
		rows = append(rows, []xlsxCell{
			{value: fmt.Sprint(i + 1), style: 3}, {value: file.FileName, style: 1},
			{value: file.FileType, style: 3}, {value: file.MimeType, style: 1},
			{value: item.Category, style: 3}, {value: item.Caption, style: 1},
			{value: createdAt, style: 1},
		})
	}
	return makeXLSX(rows)
}

func makeXLSX(rows [][]xlsxCell) ([]byte, error) {
	var output bytes.Buffer
	zw := zip.NewWriter(&output)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="첨부자료 색인" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="3"><font><sz val="10"/><name val="Noto Sans CJK KR"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="10"/><name val="Noto Sans CJK KR"/></font><font><sz val="10"/><name val="Noto Sans CJK KR"/></font></fonts><fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF145263"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border/><border><left style="thin"><color rgb="FFD9E2E8"/></left><right style="thin"><color rgb="FFD9E2E8"/></right><top style="thin"><color rgb="FFD9E2E8"/></top><bottom style="thin"><color rgb="FFD9E2E8"/></bottom></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0" applyAlignment="1"><alignment vertical="top" wrapText="1"/></xf><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyAlignment="1"><alignment horizontal="center" vertical="center" wrapText="1"/></xf><xf numFmtId="0" fontId="2" fillId="0" borderId="1" xfId="0" applyAlignment="1"><alignment horizontal="center" vertical="top" wrapText="1"/></xf></cellXfs></styleSheet>`,
	}
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetPr><pageSetUpPr fitToPage="1"/></sheetPr><sheetViews><sheetView showGridLines="0" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="7" customWidth="1"/><col min="2" max="2" width="28" customWidth="1"/><col min="3" max="5" width="18" customWidth="1"/><col min="6" max="6" width="55" customWidth="1"/><col min="7" max="7" width="25" customWidth="1"/></cols><sheetData>`)
	for rowIndex, row := range rows {
		height := 26
		if rowIndex > 0 {
			height = 42
		}
		fmt.Fprintf(&sheet, `<row r="%d" ht="%d" customHeight="1">`, rowIndex+1, height)
		for colIndex, cell := range row {
			ref := fmt.Sprintf("%s%d", excelColumn(colIndex+1), rowIndex+1)
			fmt.Fprintf(&sheet, `<c r="%s" t="inlineStr" s="%d"><is><t xml:space="preserve">%s</t></is></c>`, ref, cell.style, xmlEscape(cell.value))
		}
		sheet.WriteString(`</row>`)
	}
	lastRow := len(rows)
	sheet.WriteString(`</sheetData><autoFilter ref="A1:G` + fmt.Sprint(lastRow) + `"/><pageMargins left="0.25" right="0.25" top="0.5" bottom="0.5" header="0.2" footer="0.2"/><pageSetup paperSize="9" orientation="landscape" fitToWidth="1" fitToHeight="0"/></worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	order := []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"}
	for _, name := range order {
		writer, err := zw.Create(filepath.ToSlash(name))
		if err != nil {
			return nil, err
		}
		if _, err := writer.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func excelColumn(n int) string {
	var result string
	for n > 0 {
		n--
		result = string(rune('A'+n%26)) + result
		n /= 26
	}
	return result
}

func xmlEscape(value string) string {
	var output bytes.Buffer
	_ = xml.EscapeText(&output, []byte(value))
	return output.String()
}
