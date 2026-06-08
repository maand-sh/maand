// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package kv provides an in-memory key-value store synced to maand.db.
package kv

import (
	"fmt"
	"strings"
	"sync"
)

// MaxVersionsToKeep is how many historical versions PurgeStaleVersions retains per key.
const MaxVersionsToKeep = 7

// Store holds the working set of KV data for one maand command transaction.
type Store struct {
	mu         sync.RWMutex
	namespaces map[string]map[string]*Entry
}

// NewStore creates an empty in-memory store.
func NewStore() *Store {
	return &Store{
		namespaces: make(map[string]map[string]*Entry),
	}
}

func (s *Store) namespaceMap(namespace string) map[string]*Entry {
	ns, ok := s.namespaces[namespace]
	if !ok {
		ns = make(map[string]*Entry)
		s.namespaces[namespace] = ns
	}
	return ns
}

// Put sets or updates a key. Deleted keys are revived with a new higher version.
func (s *Store) Put(namespace, key, value string, ttl int) {
	s.putValue(namespace, key, value, ttl, true)
}

func (s *Store) putValue(namespace, key, value string, ttl int, trimValue bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if trimValue {
		value = strings.TrimSpace(value)
	}
	ns := s.namespaceMap(namespace)

	entry, exists := ns[key]
	if !exists {
		ns[key] = &Entry{
			Value:   value,
			Version: 1,
			TTL:     ttl,
			Deleted: 0,
			Changed: true,
		}
		return
	}
	if entry.Deleted == 1 {
		entry.Value = value
		entry.TTL = ttl
		entry.Deleted = 0
		entry.Version++
		entry.Changed = true
		return
	}

	changed := false
	if entry.TTL != ttl {
		entry.TTL = ttl
		changed = true
	}
	if entry.Value != value {
		entry.Value = value
		changed = true
	}
	if changed {
		entry.Version++
		entry.Changed = true
	}
}

// Delete marks a key deleted and bumps its version.
func (s *Store) Delete(namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.namespaces[namespace]
	if !ok {
		return fmt.Errorf("%w: %s", ErrNamespaceNotFound, namespace)
	}

	entry, ok := ns[key]
	if !ok {
		return ErrNotFound
	}

	entry.Deleted = 1
	entry.Changed = true
	entry.Version++
	return nil
}

// Get returns the current value for a key.
func (s *Store) Get(namespace, key string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.namespaces[namespace]
	if !ok {
		return Entry{}, ErrNotFound
	}

	entry, ok := ns[key]
	if !ok || entry.Deleted == 1 {
		return Entry{}, ErrNotFound
	}

	return *entry, nil
}

// GetKeys lists non-deleted keys in a namespace.
func (s *Store) GetKeys(namespace string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, ok := s.namespaces[namespace]
	if !ok {
		return []string{}, nil
	}

	keys := make([]string, 0, len(ns))
	for key, entry := range ns {
		if entry.Deleted != 1 {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// PurgeNamespace marks every key in namespace deleted.
func (s *Store) PurgeNamespace(namespace string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, ok := s.namespaces[namespace]
	if !ok {
		return
	}
	for _, entry := range ns {
		if entry.Deleted == 1 {
			continue
		}
		entry.Deleted = 1
		entry.Changed = true
		entry.Version++
	}
}

// HasPendingChanges reports whether any entry needs to be written to the database.
func (s *Store) HasPendingChanges() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, keys := range s.namespaces {
		for _, entry := range keys {
			if entry.Changed {
				return true
			}
		}
	}
	return false
}

// ListNamespaces returns all namespace names in the store.
func (s *Store) ListNamespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespaces := make([]string, 0, len(s.namespaces))
	for name := range s.namespaces {
		namespaces = append(namespaces, name)
	}
	return namespaces
}

// GetNamespaces is deprecated; use ListNamespaces.
func (s *Store) GetNamespaces() []string {
	return s.ListNamespaces()
}
