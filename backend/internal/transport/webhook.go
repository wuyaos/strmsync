package transport

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookHandler 处理外部网盘事件的接入入口（占位）
type WebhookHandler struct{}

// NewWebhookHandler 创建 WebhookHandler
func NewWebhookHandler() *WebhookHandler {
	return &WebhookHandler{}
}

// HandleCloudDriveWebhook 处理 CloudDrive2/Webhook 事件（占位）
func (h *WebhookHandler) HandleCloudDriveWebhook(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Webhook 处理未实现"})
}
