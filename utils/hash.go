// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"maand/bucket"
)

func CalculateFileMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("%w: file %s %w", bucket.ErrUnexpectedError, filePath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("%w: file %s %w", bucket.ErrUnexpectedError, filePath, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CalculateDirMD5(folderPath string) (string, error) {
	var files []string
	// Walk through the folder and collect file paths.
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
		}
		if strings.Contains(path, "__pycache__") {
			return nil
		}
		if !info.IsDir() {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort file paths to ensure a consistent order.
	sort.Strings(files)

	fileMD5Map := make(map[string]string)
	var wg sync.WaitGroup

	// Limit concurrency to the number of CPUs.
	sem := make(chan struct{}, runtime.NumCPU())
	var mu sync.Mutex

	// Compute MD5 of each file concurrently.
	for _, file := range files {
		wg.Add(1)
		sem <- struct{}{}

		go func(f string) {
			defer func() {
				wg.Done()
				<-sem
			}()

			md5Str, err := CalculateFileMD5(f)
			if err != nil {
				log.Fatalln(err)
				return
			}

			mu.Lock()
			fileMD5Map[f] = md5Str
			mu.Unlock()
		}(file)
	}

	wg.Wait()

	// Update the global MD5 hash in sorted order.
	globalHash := md5.New()
	for _, file := range files {
		if md5Str, ok := fileMD5Map[file]; ok {
			globalHash.Write([]byte(md5Str))
		}
	}

	return hex.EncodeToString(globalHash.Sum(nil)), nil
}

func MD5Content(content []byte) (string, error) {
	hash := md5.New()
	hash.Write(content)
	md5Hash := hash.Sum(nil)
	return hex.EncodeToString(md5Hash), nil
}
