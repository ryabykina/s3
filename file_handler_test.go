package main

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"
	"testing"
)

// nopCloserFile wraps bytes.Reader to satisfy the multipart.File interface.
type nopCloserFile struct{ *bytes.Reader }

func (nopCloserFile) Close() error { return nil }

func TestFileHandler_uploadFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir + "/",
		host:       "http://localhost",
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", "image/jpeg")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fake jpeg content"))
	writer.Close()

	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}

	fileHeader := form.File["file"][0]

	url, err := fileHandler.uploadFile("user123", fileHeader)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasPrefix(url, "http://localhost/") {
		t.Errorf("expected URL to start with http://localhost/, got: %s", url)
	}
}

func TestFileHandler_uploadFile_ContentTypeIsNull(t *testing.T) {
	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir + "/",
		host:       "http://localhost",
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", "")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = part.Write([]byte("fake jpeg content"))
	writer.Close()

	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}

	fileHeader := form.File["file"][0]

	url, err := fileHandler.uploadFile("user123", fileHeader)

	if err == nil {
		t.Fatal("expected an error when Content-Type is empty, got nil")
	}

	if url != "" {
		t.Errorf("expected empty URL on error, got: %s", url)
	}
}

func TestFileHandler_uploadFile_WrongFileExtension(t *testing.T) {
	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir + "/",
		host:       "http://localhost",
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.pdf"`)
	h.Set("Content-Type", "application/pdf")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fake pdf content"))
	writer.Close()

	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}

	fileHeader := form.File["file"][0]

	url, err := fileHandler.uploadFile("user123", fileHeader)

	if err == nil {
		t.Fatal("expected an error for unsupported content type, got nil")
	}

	if url != "" {
		t.Errorf("expected empty URL on error, got: %s", url)
	}
}

func TestFileHandler_uploadFile_OpenFileError(t *testing.T) {
	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir + "/",
		host:       "http://localhost",
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
	h.Set("Content-Type", "image/jpeg")

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fake jpeg content"))
	writer.Close()

	reader := multipart.NewReader(&buf, writer.Boundary())
	// maxMemory=0 forces the file to be written to a temp file on disk instead of kept in memory
	form, err := reader.ReadForm(0)
	if err != nil {
		t.Fatal(err)
	}

	fileHeader := form.File["file"][0]

	// Delete the temp file so that fileHeader.Open() fails inside uploadFile
	form.RemoveAll()

	url, err := fileHandler.uploadFile("user123", fileHeader)

	if err == nil {
		t.Fatal("expected an error when file cannot be opened, got nil")
	}

	if !strings.Contains(err.Error(), "Error happened while opening the file") {
		t.Errorf("unexpected error message: %v", err)
	}

	if url != "" {
		t.Errorf("expected empty URL on error, got: %s", url)
	}
}

func TestFileHandler_saveFile_Success(t *testing.T) {

	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir,
		host:       "http://localhost",
	}

	fileContent := []byte("fake jpeg content")
	file := &nopCloserFile{bytes.NewReader(fileContent)}

	filePath, err := fileHandler.saveFile(file, "jpg", tmpDir)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !strings.HasSuffix(filePath, ".jpg") {
		t.Errorf("expected file path to end with .jpg, got: %s", filePath)
	}

	if !strings.HasPrefix(filePath, tmpDir) {
		t.Errorf("expected file path to be inside %s, got: %s", tmpDir, filePath)
	}

	saved, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("could not read saved file: %v", err)
	}

	if !bytes.Equal(saved, fileContent) {
		t.Errorf("expected saved content %q, got %q", fileContent, saved)
	}
}

func TestFileHandler_saveFileFailedByGeneratingFileName(t *testing.T) {
	tmpDir := t.TempDir()

	fileHandler := &FileHandler{
		storageDir: tmpDir + "/",
		host:       "http://localhost",
		nameGen: func() (string, error) {
			return "", fmt.Errorf("name generation failed")
		},
	}

	file := &nopCloserFile{bytes.NewReader([]byte("fake jpeg content"))}

	filePath, err := fileHandler.saveFile(file, "jpg", tmpDir)

	if err == nil {
		t.Fatal("expected an error when file name generation fails, got nil")
	}

	if !strings.Contains(err.Error(), "An error happened during generating a file name") {
		t.Errorf("unexpected error message: %v", err)
	}

	if filePath != "" {
		t.Errorf("expected empty file path on error, got: %s", filePath)
	}
}
