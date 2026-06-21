// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"fmt"
	"os"

	"maand/bucket"
)

const (
	prometheusConfigFile    = "prometheus.yml"
	prometheusConfigTplFile = "prometheus.yml.tpl"
)

// ValidatePrometheusServerFiles ensures a job does not define both prometheus.yml and prometheus.yml.tpl.
func ValidatePrometheusServerFiles(jobName string) error {
	jobDir := JobFilePath(jobName)
	staticPath := fmt.Sprintf("%s/%s", jobDir, prometheusConfigFile)
	tplPath := fmt.Sprintf("%s/%s", jobDir, prometheusConfigTplFile)

	staticExists := fileExists(staticPath)
	tplExists := fileExists(tplPath)
	if staticExists && tplExists {
		return fmt.Errorf("%w: job %s cannot define both %s and %s",
			bucket.ErrInvalidJob, jobName, prometheusConfigFile, prometheusConfigTplFile)
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
