package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type UploadResponse struct {
	Result   ImageResult `json:"result"`
	Success  bool        `json:"success"`
	Errors   []string    `json:"errors"`
	Messages []string    `json:"messages"`
}

type ImageResult struct {
	ID                string            `json:"id"`
	Filename          string            `json:"filename"`
	Metadata          map[string]string `json:"metadata"`
	Uploaded          time.Time         `json:"uploaded"`
	RequireSignedURLs bool              `json:"requireSignedURLs"`
	Variants          []string          `json:"variants"`
}

type FileHandler struct {
	storageDir string
	host       string
	nameGen    func() (string, error)
}

func NewFileHandler(host string) *FileHandler {
	viper.SetConfigName("storage")
	viper.AddConfigPath("$HOME/GolandProjects/s3/config")
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("s3")

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("failed to read config: %w", err))
	}

	fh := &FileHandler{host: host}

	fh.storageDir = viper.GetString("storageDir")

	return fh
}

func (fh *FileHandler) userDirName(userId string) string {
	sum := sha256.Sum256([]byte(userId))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func (fh *FileHandler) getFile(dirName string, fileId string) ([]byte, error) {
	dirPath := fh.storageDir + dirName + "/"

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))
		if nameWithoutExt == fileId {
			return os.ReadFile(dirPath + name)
		}
	}

	return nil, fmt.Errorf("file with id %s not found in %s", fileId, dirPath)
}

func (fh *FileHandler) uploadFile(userId string, fileHeader *multipart.FileHeader) (UploadResponse, error) {
	// create directory if it doesn't exist
	userDirName := fh.userDirName(userId)
	dirPath := fh.storageDir + userDirName

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		os.Mkdir(dirPath, 0775)
	}

	contentType := fileHeader.Header.Get("Content-Type")

	if contentType == "" {
		return UploadResponse{}, fmt.Errorf("There is no Content-Type in the file %v", fileHeader.Filename)
	}

	fileExtension, err := fh.getExtensionFromContenType(contentType)

	if err != nil {
		return UploadResponse{}, err
	}

	file, err := fileHeader.Open()

	if err != nil {
		return UploadResponse{}, fmt.Errorf("Error happened while opening the file %v", fileHeader.Filename)
	}

	defer file.Close()

	imageResult, err := fh.saveFile(file, fileHeader.Filename, fileExtension, userDirName)

	if err != nil {
		return UploadResponse{}, fmt.Errorf("Error happened while opening the file %v", fileHeader.Filename)
	}

	uploadResponse := UploadResponse{
		Result:   *imageResult,
		Success:  true,
		Errors:   make([]string, 0),
		Messages: make([]string, 0),
	}

	//return fh.host + "/" + filePath, nil
	return uploadResponse, nil
}

func (fh *FileHandler) saveFile(file multipart.File, fileName string, fileExtension string, userDirName string) (*ImageResult, error) {

	newFileName, err := fh.generateFileName()

	if err != nil {
		return &ImageResult{}, fmt.Errorf("An error happened during generating a file name: %w", err)
	}

	newFileName = newFileName + "." + fileExtension

	fullFileName := fh.storageDir + "/" + userDirName + "/" + newFileName

	fo, err := os.Create(fullFileName)
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	if err != nil {
		return &ImageResult{}, fmt.Errorf("Error while creating a file %w", err)
	}

	for {
		content := make([]byte, 1024)
		n, err := file.Read(content)
		if err != nil && err != io.EOF {
			return &ImageResult{}, fmt.Errorf("Error while reading the file %w", err)
		}

		if n == 0 {
			break
		}

		if _, err := fo.Write(content[:n]); err != nil {
			return &ImageResult{}, fmt.Errorf("Error while writing in the file %w", err)
		}
	}

	metadata := make(map[string]string)
	metadata["key"] = "value"

	id := newFileName[0 : len(newFileName)-4]
	publicVariant := fh.getVariant("public", userDirName, id)

	variants := make([]string, 1)
	variants[0] = publicVariant

	imageResult := &ImageResult{
		ID:                id,
		Filename:          fileName,
		Metadata:          metadata,
		Uploaded:          time.Now(),
		RequireSignedURLs: false,
		Variants:          variants,
	}

	return imageResult, nil

}

func (fh *FileHandler) generateFileName() (string, error) {
	if fh.nameGen != nil {
		return fh.nameGen()
	}

	timestampStr := fmt.Sprintf("%d", time.Now().UnixNano())

	uuid, err := uuid.FromBytes([]byte(timestampStr[:16]))

	if err != nil {
		return "", fmt.Errorf("An error while generating UUID as a file name: %w", err)
	}

	return uuid.String(), nil
}

func (fh *FileHandler) getExtensionFromContenType(contentType string) (string, error) {
	mimeType := strings.Split(contentType, ";")[0] // Ignore params like charset
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return "jpg", nil
	case "image/png":
		return "png", nil
	case "image/gif":
		return "gif", nil
	default:
		return "", fmt.Errorf("Impossible to define an extension fro the contentType %v", contentType)
	}
}

func (fh *FileHandler) getVariant(variantType string, userDirName string, id string) string {
	return fh.host + "/store/" + userDirName + "/" + id + "/" + variantType
}
