// Package utils provides fast hashing helpers for file change detection.
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const fastHashChunkSize = 1 << 20 // 1MB

// FastFileHash computes a fast hash using:
//   - first 1MB
//   - last 1MB
//   - file size
// This is a high-performance heuristic for change detection.
//
// Example:
//  h, err := FastFileHash("movie.mkv")
func FastFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()

	h := sha256.New()
	if _, err := fmt.Fprintf(h, "%d|", size); err != nil {
		return "", err
	}

	// Read head
	headSize := min64(size, fastHashChunkSize)
	if headSize > 0 {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		if err := writeChunk(h, f, headSize); err != nil {
			return "", err
		}
	}

	// Read tail
	if size > fastHashChunkSize {
		if _, err := f.Seek(size-fastHashChunkSize, io.SeekStart); err != nil {
			return "", err
		}
		if err := writeChunk(h, f, fastHashChunkSize); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeChunk(w io.Writer, r io.Reader, n int64) error {
	if _, err := io.CopyN(w, r, n); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func min64(a int64, b int) int64 {
	if a < int64(b) {
		return a
	}
	return int64(b)
}
