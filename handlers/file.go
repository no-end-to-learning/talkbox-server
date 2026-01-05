package handlers

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	ext := strings.ToLower(filepath.Ext(header.Filename))
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

	fileInfo, err := out.Stat()
	if err != nil {
		utils.InternalError(c, "failed to get file info")
		return
	}

	utils.Success(c, gin.H{
		"url":       "/files/" + filename,
		"name":      header.Filename,
		"size":      fileInfo.Size(),
		"mime_type": header.Header.Get("Content-Type"),
	})
}

func ServeFile(c *gin.Context) {
	filename := c.Param("filename")

	// 防止路径遍历攻击：清理文件名并验证
	cleanFilename := filepath.Clean(filename)
	if cleanFilename != filepath.Base(cleanFilename) || cleanFilename == "." || cleanFilename == ".." {
		utils.BadRequest(c, "invalid filename")
		return
	}

	filePath := filepath.Join(config.Cfg.UploadDir, cleanFilename)

	// 确保最终路径在上传目录内
	absUploadDir, err := filepath.Abs(config.Cfg.UploadDir)
	if err != nil {
		utils.InternalError(c, "server configuration error")
		return
	}
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		utils.BadRequest(c, "invalid file path")
		return
	}
	if !strings.HasPrefix(absFilePath, absUploadDir+string(filepath.Separator)) {
		utils.BadRequest(c, "invalid file path")
		return
	}

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
