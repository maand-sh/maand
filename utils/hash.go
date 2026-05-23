// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"maand/bucket"
)

// HashFile returns the hex MD5 digest of a file.
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("%w: file %s: %w", bucket.ErrUnexpectedError, path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	digest := md5.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", fmt.Errorf("%w: file %s: %w", bucket.ErrUnexpectedError, path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// HashBytes returns the hex MD5 digest of content.
func HashBytes(content []byte) (string, error) {
	sum := md5.Sum(content)
	return hex.EncodeToString(sum[:]), nil
}

// HashDirectory returns a stable MD5 over sorted file hashes under root (skips __pycache__).
func HashDirectory(root string) (string, error) {
	var filePaths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, walkErr)
		}
		if strings.Contains(path, "__pycache__") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(filePaths)

	fileHashes := make(map[string]string, len(filePaths))
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, runtime.NumCPU())
	)

	for _, filePath := range filePaths {
		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			digest, err := HashFile(path)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			fileHashes[path] = digest
			mu.Unlock()
		}(filePath)
	}
	wg.Wait()
	if firstErr != nil {
		return "", firstErr
	}

	combined := md5.New()
	for _, filePath := range filePaths {
		if _, err := combined.Write([]byte(fileHashes[filePath])); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(combined.Sum(nil)), nil
}

// CalculateFileMD5 is deprecated; use HashFile.
func CalculateFileMD5(filePath string) (string, error) {
	return HashFile(filePath)
}

// CalculateDirMD5 is deprecated; use HashDirectory.
func CalculateDirMD5(folderPath string) (string, error) {
	return HashDirectory(folderPath)
}

// MD5Content is deprecated; use HashBytes.
func MD5Content(content []byte) (string, error) {
	return HashBytes(content)
}
