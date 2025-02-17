package utils

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

func MD5Content(content []byte) (string, error) {
	hash := md5.New()
	hash.Write(content)
	md5Hash := hash.Sum(nil)
	return hex.EncodeToString(md5Hash), nil
}

func UpdateHash(tx *sql.Tx, namespace, key, hash string) {
	var dbCurrentHash string
	row := tx.QueryRow("SELECT current_hash FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&dbCurrentHash)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.Exec("INSERT INTO hash (namespace, key, current_hash) VALUES (?, ?, ?)", namespace, key, hash)
		Check(err)
	} else {
		_, err = tx.Exec("UPDATE hash SET current_hash = ? WHERE namespace = ? AND key = ?", hash, namespace, key)
		Check(err)
	}
	Check(err)
}

func HashChanged(tx *sql.Tx, namespace, key string) bool {
	var dbCurrentHash, dbPreviousHash string
	row := tx.QueryRow("SELECT ifnull(current_hash, '') as current_hash, ifnull(previous_hash, '') as previous_hash FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&dbCurrentHash, &dbPreviousHash)
	if errors.Is(err, sql.ErrNoRows) || dbCurrentHash != dbPreviousHash {
		return true
	}
	return false
}

func PromoteHash(tx *sql.Tx, namespace, key string) {
	_, err := tx.Exec("UPDATE hash SET previous_hash = current_hash WHERE namespace = ? AND key = ?", namespace, key)
	Check(err)
}

func RemoveHash(tx *sql.Tx, namespace, key string) {
	_, err := tx.Exec("DELETE FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	Check(err)
}

func GetPreviousHash(tx *sql.Tx, namespace, key string) string {
	var previousHash string
	row := tx.QueryRow("SELECT ifnull(previous_hash, '') FROM hash WHERE namespace = ? AND key = ?", namespace, key)
	err := row.Scan(&previousHash)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	Check(err)
	return previousHash
}

func CalculateFileMD5(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file %s: %v", filePath, err)
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("error reading file %s: %v", filePath, err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func CalculateDirMD5(folderPath string) (string, error) {
	var files []string
	// Walk through the folder and collect file paths.
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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

	// Map to store file MD5s using the file path as the key.
	fileMD5Map := make(map[string]string)
	var wg sync.WaitGroup
	// Limit concurrency to the number of CPUs.
	sem := make(chan struct{}, runtime.NumCPU())
	var mu sync.Mutex

	// Compute MD5 of each file concurrently.
	for _, file := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(file string) {
			defer wg.Done()
			defer func() { <-sem }()
			md5Str, err := CalculateFileMD5(file)
			if err != nil {
				// Print the error and skip the file.
				fmt.Printf("Error reading file %s: %v\n", file, err)
				return
			}
			mu.Lock()
			fileMD5Map[file] = md5Str
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
