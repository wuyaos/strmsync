package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	// 读取端口配置
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// 创建 Gin 路由器
	r := gin.Default()

	// 健康检查接口
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "STRMSync is running",
		})
	})

	// 启动服务
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting STRMSync server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
