// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"maand/bucket"
	"maand/data"
	"maand/initialize"

	"github.com/stretchr/testify/require"
)

const (
	monitoringJobNodeExporter = "node_exporter"
	monitoringJobPrometheus   = "prometheus"
)

func requireDockerOnWorkers(t *testing.T) {
	t.Helper()
	requireDockerOnLocal(t)
	for _, ip := range workerIPsFromAssets(t) {
		out := strings.TrimSpace(remoteShellOutput(t, ip,
			`command -v docker >/dev/null 2>&1 && (docker compose version 2>/dev/null || docker-compose version 2>/dev/null) || echo missing`,
		))
		if out == "" || strings.Contains(out, "missing") {
			t.Skipf("docker compose not available on worker %s", ip)
		}
	}
}

func requireDockerOnLocal(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		t.Skipf("docker not available on CLI host for integration tests: %v (%s)", err, strings.TrimSpace(string(out)))
	}
}

func writeMonitoringJobs(t *testing.T) {
	t.Helper()
	writeNodeExporterJob(t)
	writePrometheusJob(t)
}

func writeNodeExporterJob(t *testing.T) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", monitoringJobNodeExporter)
	modulesDir := filepath.Join(jobDir, "_modules")
	promDir := filepath.Join(jobDir, "_prometheus")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))
	require.NoError(t, os.MkdirAll(promDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["worker"],
		"resources": {"ports": {"node_exporter_metrics_port": {}}},
		"commands": {
			"command_ready": {"executed_on": ["cli"]}
		}
	}`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(promDir, "scrape.yaml"), []byte(`
- job_name: node_exporter
  metrics_path: /metrics
  static_configs:
    - targets:
        - maand:port/node_exporter_metrics_port
`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(nodeExporterMakefile()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "config.env.tpl"), []byte(
		`METRICS_PORT={{ get "maand/bucket" "node_exporter_metrics_port" }}`+"\n",
	), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "docker-compose.yml.tpl"), []byte(nodeExporterCompose()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modulesDir, "command_ready.py"), []byte(`print("node-exporter-ready")`), 0o644))
}

func writePrometheusJob(t *testing.T) {
	t.Helper()
	jobDir := filepath.Join(bucket.WorkspaceLocation, "jobs", monitoringJobPrometheus)
	modulesDir := filepath.Join(jobDir, "_modules")
	require.NoError(t, os.MkdirAll(modulesDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "manifest.json"), []byte(`{
		"version": "1.0.0",
		"selectors": ["worker"],
		"resources": {"ports": {"prometheus_http_port": {}}},
		"commands": {
			"command_pre_deploy": {
				"executed_on": ["pre_deploy"],
				"demands": {
					"job": "node_exporter",
					"command": "command_ready",
					"config": {"min_version": "1.0.0"}
				}
			}
		}
	}`), 0o644))

	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "Makefile"), []byte(prometheusMakefile()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "config.env.tpl"), []byte(
		`PROMETHEUS_PORT={{ get "maand/bucket" "prometheus_http_port" }}`+"\n",
	), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "docker-compose.yml.tpl"), []byte(prometheusCompose()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "prometheus.yml.tpl"), []byte(prometheusScrapeConfig()), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(modulesDir, "command_pre_deploy.py"), []byte(`print("prometheus-pre-deploy-ok")`), 0o644))
}

func setupMonitoringIntegrationBucket(t *testing.T) {
	t.Helper()
	requireIntegrationAssets(t)
	resetIntegrationBucket(t)
	require.NoError(t, initialize.Execute())
	installAssets(t)
	writeMonitoringJobs(t)
	executeBuild(t)
}

func jobDeploymentSeq(t *testing.T, job string) int {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var seq int
	require.NoError(t, db.QueryRow(
		`SELECT ifnull((SELECT DISTINCT deployment_seq FROM allocations WHERE job = ?), 0)`,
		job,
	).Scan(&seq))
	return seq
}

func jobAssignedPort(t *testing.T, job, portName string) string {
	t.Helper()
	db, err := data.OpenDatabase(true)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	var port string
	require.NoError(t, db.QueryRow(
		`SELECT port FROM job_ports
		 WHERE job_id = (SELECT job_id FROM job WHERE name = ?) AND name = ?`,
		job, portName,
	).Scan(&port))
	require.NotEmpty(t, port)
	return port
}

func nodeExporterMakefile() string {
	return `.PHONY: start stop restart test logs

COMPOSE := docker compose -f docker-compose.yml
LOG_TAIL ?= 100

include config.env
export

start:
	$(COMPOSE) up -d --remove-orphans

stop:
	$(COMPOSE) down --remove-orphans

restart: stop start

logs:
	$(COMPOSE) logs --tail=$(LOG_TAIL) --no-color

test:
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		curl -sf "http://127.0.0.1:$(METRICS_PORT)/metrics" | head -5 | grep -Eq '^# (HELP|TYPE)|^node_' && \
		{ echo node-exporter-metrics-ok; exit 0; }; \
		sleep 3; \
	done; exit 1
`
}

func prometheusMakefile() string {
	return `.PHONY: start stop restart test logs

COMPOSE := docker compose -f docker-compose.yml
LOG_TAIL ?= 100

include config.env
export

start:
	$(COMPOSE) up -d --remove-orphans

stop:
	$(COMPOSE) down --remove-orphans

restart: stop start

logs:
	$(COMPOSE) logs --tail=$(LOG_TAIL) --no-color

test:
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		curl -sf "http://127.0.0.1:$(PROMETHEUS_PORT)/api/v1/targets" | python3 -c 'import json,sys; data=json.load(sys.stdin); targets=[t for t in data.get("data",{}).get("activeTargets",[]) if t.get("labels",{}).get("job")=="node_exporter"]; assert targets, "no node_exporter targets"; bad=[t for t in targets if t.get("health")!="up"]; assert not bad, bad; print("prometheus-node-exporter-connected")' && exit 0; \
		sleep 3; \
	done; exit 1
`
}

func nodeExporterCompose() string {
	return `services:
  node_exporter:
    image: prom/node-exporter:v1.8.2
    restart: unless-stopped
    ports:
      - "{{ get "maand/bucket" "node_exporter_metrics_port" }}:9100"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    command:
      - --path.procfs=/host/proc
      - --path.sysfs=/host/sys
      - --path.rootfs=/rootfs
      - --web.listen-address=:9100
`
}

func prometheusCompose() string {
	return `services:
  prometheus:
    image: prom/prometheus:v2.54.1
    restart: unless-stopped
    ports:
      - "{{ get "maand/bucket" "prometheus_http_port" }}:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - ./rules:/etc/prometheus/rules:ro
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --web.enable-lifecycle
`
}

func prometheusScrapeConfig() string {
	return `global:
  scrape_interval: 5s
  evaluation_interval: 5s

scrape_configs:
{{ scrapeConfigs }}
`
}
