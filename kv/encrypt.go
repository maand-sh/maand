// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedValuePrefix = "enc:v1:"

var (
	ErrEncryptionKeyMissing = errors.New("kv encryption key is missing or invalid")
	ErrNotEncrypted         = errors.New("value is not encrypted")
	ErrDecryptFailed        = errors.New("failed to decrypt kv value")
)

// IsEncryptedValue reports whether a stored value uses the encrypted prefix.
func IsEncryptedValue(value string) bool {
	return strings.HasPrefix(value, encryptedValuePrefix)
}

// EncryptPlaintext encrypts plaintext for storage in key_value.value.
func EncryptPlaintext(plaintext string) (string, error) {
	key, err := loadEncryptionKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedValuePrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptStoredValue decrypts a value previously written by EncryptPlaintext.
func DecryptStoredValue(stored string) (string, error) {
	if !IsEncryptedValue(stored) {
		return "", ErrNotEncrypted
	}

	key, err := loadEncryptionKey()
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, encryptedValuePrefix))
	if err != nil {
		return "", ErrDecryptFailed
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrDecryptFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptFailed
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", ErrDecryptFailed
	}

	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return string(plaintext), nil
}
