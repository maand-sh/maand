// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"fmt"
	"strings"
)

// SecretJobNamespace is the KV namespace for encrypted job secrets.
func SecretJobNamespace(job string) string {
	return fmt.Sprintf("secrets/job/%s", job)
}

// IsSecretNamespace reports whether namespace holds encrypted secrets.
func IsSecretNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, "secrets/job/")
}

// PutSecret encrypts plaintext and stores it under namespace/key.
func (s *Store) PutSecret(namespace, key, plaintext string, ttl int) error {
	encrypted, err := EncryptPlaintext(plaintext)
	if err != nil {
		return err
	}
	s.putValue(namespace, key, encrypted, ttl, false)
	return nil
}

// GetSecret returns decrypted plaintext for an encrypted KV entry.
func (s *Store) GetSecret(namespace, key string) (string, error) {
	entry, err := s.Get(namespace, key)
	if err != nil {
		return "", err
	}
	return DecryptStoredValue(entry.Value)
}
