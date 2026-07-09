package pckg

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"recoverpack-server/go-api/internal/models"
)

const defaultPackageDir = "generated-packages"

type generatedEntry struct {
	name       string
	data       []byte
	sourcePath string
}

func packageDir() string {
	if configured := os.Getenv("GENERATED_PACKAGES_DIR"); configured != "" {
		return configured
	}
	return defaultPackageDir
}

// PackagePath returns a deterministic local path and rejects path traversal.
func PackagePath(projectID string) (string, error) {
	if projectID == "" || filepath.Base(projectID) != projectID || strings.ContainsAny(projectID, `/\`) {
		return "", errors.New("invalid project ID")
	}
	return filepath.Join(packageDir(), projectID+".zip"), nil
}

// Generate writes a submission-support ZIP using stored project data and originals.
func Generate(
	project *models.Project,
	files []models.ProjectFile,
	evidence []models.Evidence,
	timeline []models.TimelineEvent,
) (*models.PackageInfo, error) {
	if project == nil {
		return nil, errors.New("project is required")
	}

	zipPath, err := PackagePath(project.ID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o750); err != nil {
		return nil, fmt.Errorf("create package directory: %w", err)
	}

	entries, err := buildEntries(project, files, evidence, timeline)
	if err != nil {
		return nil, err
	}

	temp, err := os.CreateTemp(filepath.Dir(zipPath), project.ID+"-*.zip.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temporary package: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	zw := zip.NewWriter(temp)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(time.Now())
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			_ = temp.Close()
			return nil, fmt.Errorf("create ZIP entry %s: %w", entry.name, err)
		}
		if err := writeEntry(writer, entry); err != nil {
			_ = zw.Close()
			_ = temp.Close()
			return nil, fmt.Errorf("write ZIP entry %s: %w", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		_ = temp.Close()
		return nil, fmt.Errorf("finalize ZIP: %w", err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("close ZIP: %w", err)
	}
	if err := os.Rename(tempPath, zipPath); err != nil {
		return nil, fmt.Errorf("publish ZIP: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.name)
	}
	return &models.PackageInfo{
		ProjectID:   project.ID,
		PackageURL:  fmt.Sprintf("/api/projects/%s/download", project.ID),
		Contents:    names,
		GeneratedAt: time.Now(),
	}, nil
}

func buildEntries(
	project *models.Project,
	files []models.ProjectFile,
	evidence []models.Evidence,
	timeline []models.TimelineEvent,
) ([]generatedEntry, error) {
	summary := buildSummary(project, files, evidence, timeline)
	indexCSV, err := buildAttachmentIndex(files, evidence)
	if err != nil {
		return nil, err
	}
	timelineCSV, err := buildTimelineCSV(timeline)
	if err != nil {
		return nil, err
	}
	verificationCSV, err := buildVerificationCSV(files)
	if err != nil {
		return nil, err
	}
	rawJSON, err := json.MarshalIndent(struct {
		Project  *models.Project        `json:"project"`
		Files    []models.ProjectFile   `json:"files"`
		Evidence []models.Evidence      `json:"evidence"`
		Timeline []models.TimelineEvent `json:"timeline"`
	}{project, files, evidence, timeline}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal package data: %w", err)
	}

	entries := []generatedEntry{
		{name: "00_안내문.txt", data: utf8BOM("리커버팩 제출 보조 자료입니다.\n공식 서류나 보상 가능 여부를 판단하는 문서가 아닙니다.\nAI 생성 문구와 시간 정보는 제출 전에 반드시 사용자가 확인해야 합니다.\n")},
		{name: "01_접수용_요약.txt", data: utf8BOM(summary)},
		{name: "02_첨부자료_색인.csv", data: indexCSV},
		{name: "05_피해타임라인.csv", data: timelineCSV},
		{name: "06_복붙용_피해설명문.txt", data: utf8BOM(project.Description + "\n")},
		{name: "08_원본파일_검증목록.csv", data: verificationCSV},
		{name: "09_구조화데이터.json", data: rawJSON},
	}
	entries = append(entries, originalEntries(files, evidence)...)

	manifest, err := buildManifest(entries)
	if err != nil {
		return nil, err
	}
	return append(entries, generatedEntry{name: "10_패키지파일_SHA256.csv", data: manifest}), nil
}

func buildSummary(project *models.Project, files []models.ProjectFile, evidence []models.Evidence, timeline []models.TimelineEvent) string {
	return fmt.Sprintf(
		"리커버팩 피해 증빙 요약\n\n프로젝트명: %s\n재난 유형: %s\n발생 일시: %s\n위치: %s\n등록 자료: %d건\n분류된 증빙: %d건\n타임라인: %d건\n\n피해 설명\n%s\n",
		project.Title, project.DamageType, project.OccurredAt, project.Location,
		len(files), len(evidence), len(timeline), project.Description,
	)
}

func buildAttachmentIndex(files []models.ProjectFile, evidence []models.Evidence) ([]byte, error) {
	byFileID := make(map[string]models.Evidence, len(evidence))
	for _, item := range evidence {
		byFileID[item.FileID] = item
	}
	rows := [][]string{{"순번", "파일ID", "파일명", "파일유형", "MIME", "AI분류", "설명", "등록시각"}}
	for i, file := range files {
		item := byFileID[file.ID]
		rows = append(rows, []string{
			fmt.Sprint(i + 1), file.ID, file.FileName, file.FileType, file.MimeType,
			item.Category, item.Caption, file.CreatedAt.Format(time.RFC3339),
		})
	}
	return encodeCSV(rows)
}

func buildTimelineCSV(events []models.TimelineEvent) ([]byte, error) {
	sorted := append([]models.TimelineEvent(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].EventDate < sorted[j].EventDate })
	rows := [][]string{{"순번", "일시", "제목", "내용"}}
	for i, event := range sorted {
		rows = append(rows, []string{fmt.Sprint(i + 1), event.EventDate, event.Title, event.Description})
	}
	return encodeCSV(rows)
}

func buildVerificationCSV(files []models.ProjectFile) ([]byte, error) {
	rows := [][]string{{"파일ID", "파일명", "원본URL", "SHA-256", "검증상태"}}
	for _, file := range files {
		status := "원본 바이너리 미보관 - 해시 생성 불가"
		if file.StoragePath != "" && file.SHA256 != "" {
			status = "서버 보관 원본 해시 생성 완료"
		}
		rows = append(rows, []string{
			file.ID, file.FileName, file.FileURL, file.SHA256, status,
		})
	}
	return encodeCSV(rows)
}

func buildManifest(entries []generatedEntry) ([]byte, error) {
	rows := [][]string{{"패키지 내부 파일", "SHA-256"}}
	for _, entry := range entries {
		checksum, err := entryChecksum(entry)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", entry.name, err)
		}
		rows = append(rows, []string{entry.name, checksum})
	}
	return encodeCSV(rows)
}

func originalEntries(files []models.ProjectFile, evidence []models.Evidence) []generatedEntry {
	categoryByFile := make(map[string]string, len(evidence))
	for _, item := range evidence {
		categoryByFile[item.FileID] = sanitizeZipName(item.Category)
	}
	var entries []generatedEntry
	for _, file := range files {
		if file.StoragePath == "" {
			continue
		}
		prefixLength := min(8, len(file.ID))
		name := file.ID[:prefixLength] + "_" + sanitizeZipName(file.FileName)
		folder := "03_피해사진_원본"
		if file.FileType == "receipt" || file.FileType == "estimate" {
			folder = "07_영수증_견적서"
		}
		entries = append(entries, generatedEntry{
			name: folder + "/" + name, sourcePath: file.StoragePath,
		})
		if category := categoryByFile[file.ID]; category != "" && strings.HasPrefix(file.MimeType, "image/") {
			entries = append(entries, generatedEntry{
				name:       "04_피해사진_AI분류본/" + category + "/" + name,
				sourcePath: file.StoragePath,
			})
		}
	}
	return entries
}

func sanitizeZipName(value string) string {
	value = filepath.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.ReplaceAll(value, "..", "_")
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		return "unnamed"
	}
	return value
}

func writeEntry(writer io.Writer, entry generatedEntry) error {
	if entry.sourcePath == "" {
		_, err := writer.Write(entry.data)
		return err
	}
	source, err := os.Open(entry.sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = io.Copy(writer, source)
	return err
}

func entryChecksum(entry generatedEntry) (string, error) {
	hasher := sha256.New()
	if err := writeEntry(hasher, entry); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func encodeCSV(rows [][]string) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(rows); err != nil {
		return nil, fmt.Errorf("encode CSV: %w", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV: %w", err)
	}
	return buffer.Bytes(), nil
}

func utf8BOM(value string) []byte {
	return append([]byte{0xEF, 0xBB, 0xBF}, []byte(value)...)
}
