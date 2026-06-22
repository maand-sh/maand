// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"maand/data"
	"maand/kv"
)

const (
	orderSourceKV      = "kv"
	orderSourceDefault = "default"
)

// ResolvedDeployOrder is the allocation order used for a rollout phase.
type ResolvedDeployOrder struct {
	Ordered    []string
	FullOrder  string
	Source     string
}

func jobKVNamespace(job string) string {
	return kv.JobCatalogNamespace(job)
}

func readDeployOrderKV(job string) (string, bool) {
	store := kv.GetKVStore()
	if store == nil {
		return "", false
	}
	item, err := store.Get(jobKVNamespace(job), kv.DeployOrderKey)
	if err != nil || strings.TrimSpace(item.Value) == "" {
		return "", false
	}
	return strings.TrimSpace(item.Value), true
}

func parseOrderList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func validateDeployOrderList(job string, orderList []string, activeSet map[string]struct{}) (bool, string) {
	if len(orderList) == 0 && len(activeSet) > 0 {
		return false, "deploy_order is empty"
	}
	seen := make(map[string]struct{}, len(orderList))
	for _, ip := range orderList {
		if _, dup := seen[ip]; dup {
			return false, fmt.Sprintf("deploy_order duplicate %s", ip)
		}
		seen[ip] = struct{}{}
		if _, ok := activeSet[ip]; !ok {
			return false, fmt.Sprintf("deploy_order unknown allocation %s", ip)
		}
	}
	for ip := range activeSet {
		if _, ok := seen[ip]; !ok {
			return false, fmt.Sprintf("deploy_order missing allocation %s", ip)
		}
	}
	return true, ""
}

func defaultCatalogOrder(tx *sql.Tx, job string, candidates []string) ([]string, error) {
	catalog, err := data.GetNonRemovedAllocationsOrdered(tx, job)
	if err != nil {
		return nil, err
	}
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, ip := range candidates {
		candidateSet[ip] = struct{}{}
	}
	ordered := make([]string, 0, len(candidates))
	for _, ip := range catalog {
		if _, ok := candidateSet[ip]; ok {
			ordered = append(ordered, ip)
		}
	}
	for _, ip := range candidates {
		found := false
		for _, existing := range ordered {
			if existing == ip {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, ip)
		}
	}
	return ordered, nil
}

func orderByList(candidates []string, orderList []string) []string {
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, ip := range candidates {
		candidateSet[ip] = struct{}{}
	}
	ordered := make([]string, 0, len(candidates))
	for _, ip := range orderList {
		if _, ok := candidateSet[ip]; ok {
			ordered = append(ordered, ip)
		}
	}
	return ordered
}

// ResolveDeployOrder picks rollout order for candidates using deploy_order KV when valid.
func ResolveDeployOrder(tx *sql.Tx, job string, candidates []string) (ResolvedDeployOrder, error) {
	if len(candidates) == 0 {
		return ResolvedDeployOrder{}, nil
	}

	activeSet := make(map[string]struct{}, len(candidates))
	for _, ip := range candidates {
		activeSet[ip] = struct{}{}
	}

	defaultOrdered, err := defaultCatalogOrder(tx, job, candidates)
	if err != nil {
		return ResolvedDeployOrder{}, err
	}

	rawOrder, ok := readDeployOrderKV(job)
	if !ok {
		return ResolvedDeployOrder{
			Ordered:   defaultOrdered,
			FullOrder: strings.Join(defaultOrdered, ","),
			Source:    orderSourceDefault,
		}, nil
	}

	orderList := parseOrderList(rawOrder)
	valid, reason := validateDeployOrderList(job, orderList, activeSet)
	if !valid {
		log.Printf("deploy: job %q: %s; using default catalog order", job, reason)
		return ResolvedDeployOrder{
			Ordered:   defaultOrdered,
			FullOrder: strings.Join(defaultOrdered, ","),
			Source:    orderSourceDefault,
		}, nil
	}

	ordered := orderByList(candidates, orderList)
	return ResolvedDeployOrder{
		Ordered:   ordered,
		FullOrder: rawOrder,
		Source:    orderSourceKV,
	}, nil
}
