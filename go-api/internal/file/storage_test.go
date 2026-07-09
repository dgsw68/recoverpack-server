package file

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSaveOriginalStoresBytesAndChecksum(t *testing.T) {
	t.Setenv("UPLOAD_DIR", t.TempDir())
	content := []byte("\x89PNG\r\n\x1a\nrecoverpack-test-image")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "damage.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}

	path, size, checksum, mimeType, err := saveOriginal("project-1", "file-1", request.MultipartForm.File["files"][0])
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, content) {
		t.Fatal("stored original differs from uploaded bytes")
	}
	sum := sha256.Sum256(content)
	if checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s", checksum)
	}
	if size != int64(len(content)) || mimeType != "image/png" {
		t.Fatalf("size=%d mime=%s", size, mimeType)
	}
}
