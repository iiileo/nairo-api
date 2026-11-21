package v1

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
)

// PublicQR 根据 data 生成收款二维码 PNG（image_url 为空、仅有 payload 时用）。
func PublicQR(c *gin.Context) {
	data := strings.TrimSpace(c.Query("data"))
	if data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing_data"})
		return
	}
	if len(data) > 2048 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "data_too_long"})
		return
	}
	png, err := qrcode.Encode(data, qrcode.Medium, 512)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "qr_encode_failed"})
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "image/png", png)
}

// resolveQRURL 有图用图；否则用 payload 走本站二维码接口。
func resolveQRURL(imageURL string, payload *string) (qrURL, qrContent string) {
	imageURL = strings.TrimSpace(imageURL)
	content := ""
	if payload != nil {
		content = strings.TrimSpace(*payload)
	}
	qrContent = content
	if imageURL != "" {
		return imageURL, qrContent
	}
	if content == "" {
		return "", ""
	}
	// payload 本身已是图片地址时直接用
	if looksLikeImageURL(content) {
		return content, qrContent
	}
	return "/api/v1/public/qr.png?data=" + url.QueryEscape(content), qrContent
}

func looksLikeImageURL(s string) bool {
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "data:image/") {
		return true
	}
	if !(strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")) {
		return false
	}
	return strings.Contains(lower, ".png") ||
		strings.Contains(lower, ".jpg") ||
		strings.Contains(lower, ".jpeg") ||
		strings.Contains(lower, ".webp") ||
		strings.Contains(lower, ".gif")
}
