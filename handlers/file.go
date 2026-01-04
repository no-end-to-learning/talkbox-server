package handlers

import (
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"talkbox/config"
	"talkbox/utils"
)

func UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file uploaded")
		return
	}
	defer file.Close()

	maxSize := int64(50 * 1024 * 1024)
	if header.Size > maxSize {
		utils.BadRequest(c, "file too large (max 50MB)")
		return
	}

	ext := filepath.Ext(header.Filename)
	filename := utils.GenerateUUID() + ext
	uploadPath := filepath.Join(config.Cfg.UploadDir, filename)

	out, err := os.Create(uploadPath)
	if err != nil {
		utils.InternalError(c, "failed to save file")
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		utils.InternalError(c, "failed to save file")
		return
	}

	fileInfo, _ := out.Stat()

	utils.Success(c, gin.H{
		"url":       "/files/" + filename,
		"name":      header.Filename,
		"size":      fileInfo.Size(),
		"mime_type": header.Header.Get("Content-Type"),
	})
}

func ServeFile(c *gin.Context) {
	filename := c.Param("filename")
	filePath := filepath.Join(config.Cfg.UploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.NotFound(c, "file not found")
		return
	}

	c.File(filePath)
}

func GetFileThumbnail(c *gin.Context) {
	filename := c.Param("filename")
	widthStr := c.DefaultQuery("w", "200")
	width, _ := strconv.Atoi(widthStr)
	if width <= 0 || width > 500 {
		width = 200
	}

	filePath := filepath.Join(config.Cfg.UploadDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		utils.NotFound(c, "file not found")
		return
	}

	c.File(filePath)
}
