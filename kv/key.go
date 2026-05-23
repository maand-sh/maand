// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"crypto/rand"
	"os"
	"path"
	"sync"

	"maand/bucket"
)

const encryptionKeyFile = "kv.key"
const encryptionKeySize = 32

var encryptionKeyCache struct {
	sync.Mutex
	key    []byte
	loaded bool
}

// EnsureEncryptionKey creates secrets/kv.key when missing (32-byte AES-256 key).
func EnsureEncryptionKey() error {
	keyPath := path.Join(bucket.SecretLocation, encryptionKeyFile)
	if _, err := os.Stat(keyPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return bucket.UnexpectedError(err)
	}

	key := make([]byte, encryptionKeySize)
	if _, err := rand.Read(key); err != nil {
		return bucket.UnexpectedError(err)
	}
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return bucket.UnexpectedError(err)
	}
	return nil
}

func loadEncryptionKey() ([]byte, error) {
	encryptionKeyCache.Lock()
	defer encryptionKeyCache.Unlock()

	if encryptionKeyCache.loaded {
		if len(encryptionKeyCache.key) != encryptionKeySize {
			return nil, ErrEncryptionKeyMissing
		}
		return encryptionKeyCache.key, nil
	}

	keyPath := path.Join(bucket.SecretLocation, encryptionKeyFile)
	key, err := os.ReadFile(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrEncryptionKeyMissing
		}
		return nil, bucket.UnexpectedError(err)
	}
	if len(key) != encryptionKeySize {
		return nil, ErrEncryptionKeyMissing
	}

	encryptionKeyCache.key = key
	encryptionKeyCache.loaded = true
	return key, nil
}

// ResetEncryptionKeyCacheForTest clears the in-process encryption key cache.
func ResetEncryptionKeyCacheForTest() {
	encryptionKeyCache.Lock()
	encryptionKeyCache.key = nil
	encryptionKeyCache.loaded = false
	encryptionKeyCache.Unlock()
}
