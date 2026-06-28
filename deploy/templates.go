// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/promconfig"
	"maand/utils"
)

func transpile(tx *sql.Tx, job, workerIP string) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	jobDir := path.Join(workerDir, "jobs", job)

	var jobTemplates []string
	err := fs.WalkDir(os.DirFS(jobDir), ".", func(relPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".tpl") {
			jobTemplates = append(jobTemplates, relPath)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(jobTemplates) == 0 {
		return nil
	}

	allowedNamespaces, err := data.AllowedKVNamespacesWithUpstream(tx, job, workerIP)
	if err != nil {
		return err
	}
	funcMap := templateFuncMap(tx, job, allowedNamespaces)

	workerData, err := getWorkerData(tx, workerIP)
	if err != nil {
		return err
	}

	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return err
	}

	versions, err := allocationVersionsForWorker(tx, job, workerIP)
	if err != nil {
		return err
	}

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	templateData := AllocationData{
		AllocationID:   allocID,
		Job:            job,
		CurrentVersion: versions.CurrentVersion,
		NewVersion:     versions.NewVersion,
		WorkerData:     workerData,
		BucketPath:     fmt.Sprintf("/opt/worker/%s", bucketID),
		JobPath:        fmt.Sprintf("/opt/worker/%s/jobs/%s", bucketID, job),
	}

	for _, jobTemplate := range jobTemplates {
		if err := renderTemplate(jobDir, jobTemplate, funcMap, templateData); err != nil {
			return err
		}
	}
	return nil
}

func templateFuncMap(tx *sql.Tx, job string, allowedNamespaces []string) template.FuncMap {
	store := kv.GetKVStore()
	return template.FuncMap{
		"get": func(ns, key string) string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", ns, job))
			}
			if kv.IsSecretNamespace(ns) {
				value, err := store.GetSecret(ns, key)
				if err != nil {
					panic(err)
				}
				return value
			}
			value, err := store.Get(ns, key)
			if err != nil {
				panic(err)
			}
			return value.Value
		},
		"getOptional": func(ns, key string) string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", ns, job))
			}
			if kv.IsSecretNamespace(ns) {
				value, err := store.GetSecret(ns, key)
				if err != nil {
					if errors.Is(err, kv.ErrNotFound) {
						return ""
					}
					panic(err)
				}
				return value
			}
			value, err := store.Get(ns, key)
			if err != nil {
				if errors.Is(err, kv.ErrNotFound) {
					return ""
				}
				panic(err)
			}
			return value.Value
		},
		"keys": func(ns string) []string {
			if len(utils.Difference([]string{ns}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", ns, job))
			}
			value, err := store.GetKeys(ns)
			if err != nil {
				panic(err)
			}
			return value
		},
		"getSecret": func(key string) string {
			ns := kv.SecretJobNamespace(job)
			value, err := store.GetSecret(ns, key)
			if err != nil {
				panic(err)
			}
			return value
		},
		"split": strings.Split,
		"trim":  strings.TrimSpace,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"join":  strings.Join,
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int { return a / b },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"int": templateInt,
		"scrapeConfigs": func() string {
			if len(utils.Difference([]string{promconfig.KVNamespace}, allowedNamespaces)) > 0 {
				panic(fmt.Sprintf("%s namespace is not available for job %s", promconfig.KVNamespace, job))
			}
			yamlFragment, err := promconfig.RenderScrapeConfigsYAML(tx, nil)
			if err != nil {
				panic(err)
			}
			return yamlFragment
		},
		"ruleFiles": func() string {
			yamlFragment, err := promconfig.RenderRuleFilesYAML(tx)
			if err != nil {
				panic(err)
			}
			return yamlFragment
		},
	}
}

func templateInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			panic(err)
		}
		return i
	default:
		panic("int: expected string or integer")
	}
}

func renderTemplate(jobDir, jobTemplate string, funcMap template.FuncMap, data AllocationData) error {
	templateContent, err := os.ReadFile(path.Join(jobDir, jobTemplate))
	if err != nil {
		return err
	}

	tmpl, err := template.New("template").Funcs(funcMap).Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", jobTemplate, err)
	}

	outPath := strings.TrimSuffix(jobTemplate, path.Ext(jobTemplate))
	file, err := os.Create(path.Join(jobDir, outPath))
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("execute template %s: %w", jobTemplate, err)
	}
	return file.Close()
}
