package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// CalculateFastHash 计算文件的快速哈希（头部+尾部+大小）
// Author: STRMSync Team
func CalculateFastHash(filePath string, sampleSize int64) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()
	hasher := md5.New()

	// 写入文件大小
	hasher.Write([]byte(fmt.Sprintf("%d", fileSize)))

	if fileSize <= sampleSize {
		// 小文件：全量哈希
		if _, err := io.Copy(hasher, file); err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
	} else {
		// 大文件：头部 + 尾部哈希
		// 读取头部
		headBuf := make([]byte, sampleSize)
		n, err := file.Read(headBuf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read head: %w", err)
		}
		hasher.Write(headBuf[:n])

		// 读取尾部
		if _, err := file.Seek(-sampleSize, io.SeekEnd); err != nil {
			return "", fmt.Errorf("failed to seek tail: %w", err)
		}
		tailBuf := make([]byte, sampleSize)
		n, err = file.Read(tailBuf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("failed to read tail: %w", err)
		}
		hasher.Write(tailBuf[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateFullHash 计算文件的完整哈希（MD5）
// Author: STRMSync Team
func CalculateFullHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
