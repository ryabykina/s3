package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type WebServer struct {
	handlers map[string]Handler
}

type Handler interface {
	Handle(c *gin.Context) error
}

func main() {

	router := gin.Default()
	router.MaxMultipartMemory = 4 << 20 // 4 MiB

	fileHandler := NewFileHandler("http://localhost:8080")

	router.POST("/upload", func(c *gin.Context) {
		userId := c.PostForm("userId")
		if userId == "" {
			c.JSONP(http.StatusBadRequest, gin.H{
				"message": "The userId is required",
			})
			return
		}

		file, err := c.FormFile("file")

		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}

		uploadResponse, err := fileHandler.uploadFile(userId, file)

		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": fmt.Errorf("Ann error while uploading the file %w", err),
			})
		}

		c.JSONP(http.StatusCreated, uploadResponse)

	})

	router.GET("/:directoryName/:fileId/:type", func(c *gin.Context) {
		dirName := c.Param("directoryName")
		fileId := c.Param("fileId")
		fileType := c.Param("type")

		if dirName == "" || fileId == "" || fileType == "" {
			c.JSONP(http.StatusBadRequest, gin.H{
				"message": "Wrong URL format",
			})
			return
		}

		data, err := fileHandler.getFile(dirName, fileId)

		if err != nil {
			c.JSONP(http.StatusNotFound, gin.H{
				"message": "File not found",
			})
			return
		}

		contentType := http.DetectContentType(data)
		c.Data(http.StatusOK, contentType, data)

	})

	router.Run(":8080")

}
