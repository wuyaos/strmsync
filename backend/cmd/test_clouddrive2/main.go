// CloudDrive2客户端连接测试程序
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/strmsync/strmsync/internal/clients/clouddrive2"
)

func main() {
	// 从环境变量读取配置
	target := os.Getenv("CD2_HOST")
	if target == "" {
		target = "127.0.0.1:19798" // CloudDrive2默认gRPC端口
	}

	token := os.Getenv("CD2_TOKEN") // 可为空，GetSystemInfo不需要认证

	fmt.Printf("测试CloudDrive2连接...\n")
	fmt.Printf("  目标: %s\n", target)
	fmt.Printf("  Token: %s\n", maskToken(token))
	fmt.Println()

	// 创建客户端
	client := clouddrive2.NewClient(
		target,
		token,
		clouddrive2.WithTimeout(10*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试1: GetSystemInfo（公开接口，无需认证）
	fmt.Println("[测试1] GetSystemInfo（无需认证）")
	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		log.Printf("❌ 失败: %v\n", err)
		fmt.Println("\n可能的原因：")
		fmt.Println("  1. 端口错误（确认CloudDrive2的gRPC端口，默认19798）")
		fmt.Println("  2. CloudDrive2服务未启动")
		fmt.Println("  3. 网络/防火墙阻断")
		fmt.Println("  4. 协议不匹配（TLS vs h2c）")
		os.Exit(1)
	}
	fmt.Printf("✅ 成功\n")
	fmt.Printf("  已登录: %v\n", info.GetIsLogin())
	fmt.Printf("  用户名: %s\n", info.GetUserName())
	fmt.Printf("  系统就绪: %v\n", info.GetSystemReady())
	if msg := info.GetSystemMessage(); msg != "" {
		fmt.Printf("  系统消息: %s\n", msg)
	}

	// 检查系统状态
	if info.GetHasError() {
		fmt.Printf("  ⚠️  系统有错误\n")
		os.Exit(1)
	}
	if !info.GetSystemReady() {
		fmt.Printf("  ⚠️  系统未就绪\n")
		os.Exit(1)
	}
	fmt.Println()

	// 测试2: GetMountPoints（需要认证）
	if token == "" {
		fmt.Println("[测试2] GetMountPoints - 跳过（未提供token）")
		fmt.Println("  提示: 设置环境变量 CD2_TOKEN 可测试认证接口")
		fmt.Println()
		fmt.Println("✅ 基础连接测试通过")
		return
	}

	fmt.Println("[测试2] GetMountPoints（需要认证）")
	mounts, err := client.GetMountPoints(ctx)
	if err != nil {
		log.Printf("❌ 失败: %v\n", err)
		fmt.Println("\n可能的原因：")
		fmt.Println("  1. Token无效或已过期")
		fmt.Println("  2. Token格式错误（应为JWT格式）")
		fmt.Println("  3. 未登录CloudDrive2")
		os.Exit(1)
	}
	fmt.Printf("✅ 成功\n")
	fmt.Printf("  挂载点数量: %d\n", len(mounts.GetMountPoints()))
	for i, mp := range mounts.GetMountPoints() {
		status := "未挂载"
		if mp.GetIsMounted() {
			status = "已挂载"
		}
		fmt.Printf("  [%d] %s -> %s (%s)\n", i+1, mp.GetMountPoint(), mp.GetSourceDir(), status)
	}
	fmt.Println()

	// 测试3: GetSubFiles（需要认证，测试流式API）
	if len(mounts.GetMountPoints()) > 0 {
		// CloudDrive的虚拟路径通常是 "/" 或云盘路径（如 "/115", "/阿里云盘" 等）
		// mountPoint是本地挂载路径，不是CloudDrive的虚拟路径
		testPath := "/"
		fmt.Printf("[测试3] GetSubFiles（流式API）- 路径: %s\n", testPath)

		files, err := client.GetSubFiles(ctx, testPath, false)
		if err != nil {
			log.Printf("❌ 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ 成功\n")
		fmt.Printf("  文件数量: %d\n", len(files))
		if len(files) > 0 {
			fmt.Println("  示例文件（前3个）：")
			for i := 0; i < 3 && i < len(files); i++ {
				f := files[i]
				fmt.Printf("    [%d] %s (%s)\n", i+1, f.GetName(), formatSize(f.GetSize()))
			}
		}
		fmt.Println()
	}

	fmt.Println("✅ 所有测试通过")
	fmt.Println("\nCloudDrive2 gRPC客户端工作正常！")
}

// maskToken 遮蔽token显示
func maskToken(token string) string {
	if token == "" {
		return "<未设置>"
	}
	if len(token) <= 10 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-5:]
}

// formatSize 格式化文件大小
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
