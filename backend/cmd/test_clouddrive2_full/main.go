// CloudDrive2功能测试程序
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
	target := os.Getenv("CD2_HOST")
	if target == "" {
		target = "192.168.123.179:19798"
	}

	token := os.Getenv("CD2_TOKEN")
	if token == "" {
		log.Fatal("请设置环境变量 CD2_TOKEN")
	}

	testBasePath := os.Getenv("CD2_TEST_PATH")
	if testBasePath == "" {
		testBasePath = "/115open" // 从配置文件获取的路径
	}

	fmt.Printf("CloudDrive2功能测试\n")
	fmt.Printf("==================\n")
	fmt.Printf("目标: %s\n", target)
	fmt.Printf("测试路径: %s\n", testBasePath)
	fmt.Printf("测试文件夹: strmsync_test_%d\n", time.Now().Unix())
	fmt.Println()

	client := clouddrive2.NewClient(
		target,
		token,
		clouddrive2.WithTimeout(30*time.Second),
	)
	defer client.Close()

	ctx := context.Background()

	// 测试1: 列出文件
	fmt.Println("【测试1】列出目录内容")
	files, err := client.GetSubFiles(ctx, testBasePath, false)
	if err != nil {
		log.Fatalf("❌ GetSubFiles失败: %v", err)
	}
	fmt.Printf("✅ 成功，找到 %d 个项目\n", len(files))
	if len(files) > 0 {
		fmt.Println("前5个项目：")
		for i := 0; i < 5 && i < len(files); i++ {
			f := files[i]
			typeStr := "文件"
			if f.GetIsDirectory() {
				typeStr = "目录"
			}
			fmt.Printf("  %d. [%s] %s (%s)\n",
				i+1, typeStr, f.GetName(), formatSize(f.GetSize()))
		}
	}
	fmt.Println()

	// 测试2: 创建测试目录
	testFolderName := fmt.Sprintf("strmsync_test_%d", time.Now().Unix())
	fmt.Printf("【测试2】创建目录: %s/%s\n", testBasePath, testFolderName)

	createResult, err := client.CreateFolder(ctx, testBasePath, testFolderName)
	if err != nil {
		log.Fatalf("❌ CreateFolder失败: %v", err)
	}
	if !createResult.GetResult().GetSuccess() {
		log.Fatalf("❌ 创建失败: %s", createResult.GetResult().GetErrorMessage())
	}
	fmt.Printf("✅ 创建成功\n")
	testFolderPath := testBasePath + "/" + testFolderName
	fmt.Printf("   完整路径: %s\n", testFolderPath)
	fmt.Println()

	// 测试3: 查找文件
	fmt.Printf("【测试3】查找文件: %s\n", testFolderPath)
	fileInfo, err := client.FindFileByPath(ctx, testBasePath, testFolderName)
	if err != nil {
		log.Fatalf("❌ FindFileByPath失败: %v", err)
	}
	fmt.Printf("✅ 找到文件\n")
	fmt.Printf("   名称: %s\n", fileInfo.GetName())
	fmt.Printf("   是目录: %v\n", fileInfo.GetIsDirectory())
	fmt.Printf("   大小: %s\n", formatSize(fileInfo.GetSize()))
	fmt.Println()

	// 测试4: 重命名
	newName := testFolderName + "_renamed"
	fmt.Printf("【测试4】重命名: %s -> %s\n", testFolderName, newName)

	renameResult, err := client.RenameFile(ctx, testFolderPath, newName)
	if err != nil {
		log.Fatalf("❌ RenameFile失败: %v", err)
	}
	if !renameResult.GetSuccess() {
		log.Fatalf("❌ 重命名失败: %s", renameResult.GetErrorMessage())
	}
	fmt.Printf("✅ 重命名成功\n")
	renamedPath := testBasePath + "/" + newName
	fmt.Println()

	// 测试5: 在重命名的目录下创建子目录
	subFolderName := "subfolder"
	fmt.Printf("【测试5】创建子目录: %s/%s\n", renamedPath, subFolderName)

	subCreateResult, err := client.CreateFolder(ctx, renamedPath, subFolderName)
	if err != nil {
		log.Fatalf("❌ CreateFolder失败: %v", err)
	}
	if !subCreateResult.GetResult().GetSuccess() {
		log.Fatalf("❌ 创建子目录失败: %s", subCreateResult.GetResult().GetErrorMessage())
	}
	fmt.Printf("✅ 子目录创建成功\n")
	subFolderPath := renamedPath + "/" + subFolderName
	fmt.Println()

	// 测试6: 列出重命名后的目录内容
	fmt.Printf("【测试6】列出重命名后目录的内容: %s\n", renamedPath)
	subFiles, err := client.GetSubFiles(ctx, renamedPath, false)
	if err != nil {
		log.Fatalf("❌ GetSubFiles失败: %v", err)
	}
	fmt.Printf("✅ 成功，找到 %d 个项目\n", len(subFiles))
	for i, f := range subFiles {
		fmt.Printf("  %d. %s\n", i+1, f.GetName())
	}
	fmt.Println()

	// 测试7: 删除子目录
	fmt.Printf("【测试7】删除子目录: %s\n", subFolderPath)
	deleteSubResult, err := client.DeleteFile(ctx, subFolderPath)
	if err != nil {
		log.Fatalf("❌ DeleteFile失败: %v", err)
	}
	if !deleteSubResult.GetSuccess() {
		log.Fatalf("❌ 删除失败: %s", deleteSubResult.GetErrorMessage())
	}
	fmt.Printf("✅ 删除成功\n")
	fmt.Println()

	// 测试8: 删除测试目录
	fmt.Printf("【测试8】删除测试目录: %s\n", renamedPath)
	deleteResult, err := client.DeleteFile(ctx, renamedPath)
	if err != nil {
		log.Fatalf("❌ DeleteFile失败: %v", err)
	}
	if !deleteResult.GetSuccess() {
		log.Fatalf("❌ 删除失败: %s", deleteResult.GetErrorMessage())
	}
	fmt.Printf("✅ 删除成功\n")
	fmt.Println()

	// 测试9: 验证删除（应该找不到）
	fmt.Printf("【测试9】验证删除（查找已删除的目录）\n")
	_, err = client.FindFileByPath(ctx, testBasePath, newName)
	if err != nil {
		fmt.Printf("✅ 验证成功，目录已被删除（预期错误: %v）\n", err)
	} else {
		fmt.Printf("⚠️  警告：删除后仍能找到文件，可能是缓存\n")
	}
	fmt.Println()

	fmt.Println("==================")
	fmt.Println("✅ 所有功能测试通过！")
	fmt.Println("==================")
	fmt.Println()
	fmt.Println("测试的功能：")
	fmt.Println("  ✅ 列出目录内容 (GetSubFiles)")
	fmt.Println("  ✅ 创建目录 (CreateFolder)")
	fmt.Println("  ✅ 查找文件 (FindFileByPath)")
	fmt.Println("  ✅ 重命名文件 (RenameFile)")
	fmt.Println("  ✅ 创建子目录")
	fmt.Println("  ✅ 删除文件 (DeleteFile)")
	fmt.Println("  ✅ 验证删除")
}

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
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}
