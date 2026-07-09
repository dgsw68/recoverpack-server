package file

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize int64 = 25 << 20

var allowedMIMETypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true,
	"application/pdf": true, "application/zip": true,
	"text/plain; charset=utf-8": true, "text/csv; charset=utf-8": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
}

func uploadDir() string {
	if configured := os.Getenv("UPLOAD_DIR"); configured != "" {
		return configured
	}
	return "uploaded-files"
}

func saveOriginal(projectID, fileID string, header *multipart.FileHeader) (string, int64, string, string, error) {
	source, err := header.Open()
	if err != nil {
		return "", 0, "", "", fmt.Errorf("open upload: %w", err)
	}
	defer source.Close()

	projectDir := filepath.Join(uploadDir(), projectID)
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return "", 0, "", "", fmt.Errorf("create upload directory: %w", err)
	}
	temp, err := os.CreateTemp(projectDir, fileID+"-*.uploading")
	if err != nil {
		return "", 0, "", "", fmt.Errorf("create upload file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	buffer := make([]byte, 512)
	count, readErr := io.ReadFull(source, buffer)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		_ = temp.Close()
		return "", 0, "", "", fmt.Errorf("inspect upload: %w", readErr)
	}
	buffer = buffer[:count]
	detectedMIME := http.DetectContentType(buffer)
	if !allowedMIMETypes[detectedMIME] && !strings.HasPrefix(detectedMIME, "text/") {
		_ = temp.Close()
		return "", 0, "", "", fmt.Errorf("unsupported file type: %s", detectedMIME)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(temp, hasher)
	written, err := writer.Write(buffer)
	if err != nil {
		_ = temp.Close()
		return "", 0, "", "", fmt.Errorf("save upload header: %w", err)
	}
	remaining, err := io.Copy(writer, io.LimitReader(source, maxFileSize-int64(written)+1))
	size := int64(written) + remaining
	if err != nil {
		_ = temp.Close()
		return "", 0, "", "", fmt.Errorf("save upload: %w", err)
	}
	if size > maxFileSize {
		_ = temp.Close()
		return "", 0, "", "", errors.New("file exceeds 25 MB limit")
	}
	if err := temp.Close(); err != nil {
		return "", 0, "", "", fmt.Errorf("close upload: %w", err)
	}

	extension := strings.ToLower(filepath.Ext(filepath.Base(header.Filename)))
	if len(extension) > 10 {
		extension = ""
	}
	finalPath := filepath.Join(projectDir, fileID+extension)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return "", 0, "", "", fmt.Errorf("publish upload: %w", err)
	}
	return finalPath, size, hex.EncodeToString(hasher.Sum(nil)), detectedMIME, nil
}

func removeOriginal(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
