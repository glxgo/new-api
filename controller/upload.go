package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	uploadDir     = "/data/uploads" // 容器 WORKDIR /data, 持久 volume
	maxUploadSize = 10 << 20        // 10MB
)

// UploadImage 图片上传(使用教程配图等)。RootAuth。保存到 /data/uploads, 返回 /uploads/xxx URL。
// 前端 MD 编辑器插入图片时调用, 拿到 url 后插入 ![](url)。
func UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "未收到文件"})
		return
	}
	if file.Size > maxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "文件过大(>10MB)"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "仅支持 png/jpg/jpeg/gif/webp"})
		return
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "创建上传目录失败"})
		return
	}
	filename := uuid.New().String() + ext
	dst := filepath.Join(uploadDir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "保存失败: " + err.Error()})
		return
	}
	common.SysLog(fmt.Sprintf("image uploaded: %s (%d bytes) by user %d", filename, file.Size, c.GetInt("id")))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"url": "/uploads/" + filename, "filename": filename},
	})
}
