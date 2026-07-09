package pckg

import (
	"archive/zip"
	"encoding/csv"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"recoverpack-server/go-api/internal/models"
)

func TestGenerateCreatesDownloadableZip(t *testing.T) {
	t.Setenv("GENERATED_PACKAGES_DIR", t.TempDir())
	now := time.Date(2026, 7, 9, 15, 30, 0, 0, time.FixedZone("KST", 9*60*60))
	project := &models.Project{
		ID: "project-1", DamageType: "flood", Title: "우리 집 침수",
		Location: "서울시 테스트구", OccurredAt: "2026-07-09 15:00",
		Description: "거실 바닥의 침수 흔적을 촬영하고 정리한 자료입니다.",
	}
	files := []models.ProjectFile{{
		ID: "file-1", ProjectID: project.ID, FileName: "거실.jpg",
		FileType: "image", FileURL: "https://example.invalid/living-room.jpg",
		MimeType: "image/jpeg", CreatedAt: now,
	}}
	evidence := []models.Evidence{{
		ID: "ev-file-1", ProjectID: project.ID, FileID: "file-1",
		Category: "floor_flooding", Caption: "거실 바닥에 물 고임이 확인됩니다.",
	}}
	timeline := []models.TimelineEvent{{
		ID: "timeline-1", ProjectID: project.ID, EventDate: "2026-07-09 15:30",
		Title: "피해 촬영", Description: "거실 바닥을 촬영함",
	}}

	info, err := Generate(project, files, evidence, timeline)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if info.PackageURL != "/api/projects/project-1/download" {
		t.Fatalf("unexpected package URL: %s", info.PackageURL)
	}

	path, err := PackagePath(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("generated file is not a valid ZIP: %v", err)
	}
	defer reader.Close()

	contents := make(map[string]string)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[file.Name] = string(data)
	}

	for _, expected := range []string{
		"00_안내문.txt",
		"01_접수용_요약.txt",
		"02_첨부자료_색인.csv",
		"05_피해타임라인.csv",
		"06_복붙용_피해설명문.txt",
		"08_원본파일_검증목록.csv",
		"09_구조화데이터.json",
		"10_패키지파일_SHA256.csv",
	} {
		if _, ok := contents[expected]; !ok {
			t.Errorf("ZIP is missing %q", expected)
		}
	}
	if !strings.Contains(contents["02_첨부자료_색인.csv"], "거실.jpg") {
		t.Error("attachment index does not contain uploaded file metadata")
	}
	if !strings.Contains(contents["05_피해타임라인.csv"], "피해 촬영") {
		t.Error("timeline export does not contain timeline event")
	}

	manifest := strings.TrimPrefix(contents["10_패키지파일_SHA256.csv"], "\uFEFF")
	rows, err := csv.NewReader(strings.NewReader(manifest)).ReadAll()
	if err != nil {
		t.Fatalf("invalid manifest CSV: %v", err)
	}
	if len(rows) != 8 {
		t.Fatalf("manifest rows = %d, want 8", len(rows))
	}
}

func TestPackagePathRejectsTraversal(t *testing.T) {
	t.Setenv("GENERATED_PACKAGES_DIR", t.TempDir())
	for _, value := range []string{"", "../secret", "a/b", `a\b`} {
		if _, err := PackagePath(value); err == nil {
			t.Errorf("PackagePath(%q) should fail", value)
		}
	}
}

func TestGenerateIncludesStoredOriginal(t *testing.T) {
	t.Setenv("GENERATED_PACKAGES_DIR", t.TempDir())
	originalPath := t.TempDir() + "/original.jpg"
	original := []byte("original-evidence-bytes")
	if err := os.WriteFile(originalPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	project := &models.Project{ID: "project-original", Title: "원본 포함 테스트"}
	files := []models.ProjectFile{{
		ID: "12345678-file", ProjectID: project.ID, FileName: "피해사진.jpg",
		FileType: "image", MimeType: "image/jpeg", StoragePath: originalPath,
	}}
	evidence := []models.Evidence{{FileID: files[0].ID, Category: "floor_flooding"}}
	if _, err := Generate(project, files, evidence, nil); err != nil {
		t.Fatal(err)
	}
	path, _ := PackagePath(project.ID)
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	foundOriginal, foundClassified := false, false
	for _, entry := range reader.File {
		if strings.HasPrefix(entry.Name, "03_피해사진_원본/") {
			foundOriginal = true
		}
		if strings.HasPrefix(entry.Name, "04_피해사진_AI분류본/floor_flooding/") {
			foundClassified = true
		}
	}
	if !foundOriginal || !foundClassified {
		t.Fatalf("original=%v classified=%v", foundOriginal, foundClassified)
	}
}
