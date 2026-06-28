// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"fmt"
	"testing"

	"maand/deploy"
	"maand/jobcontrol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationDeployPrometheusNodeExporterConnected(t *testing.T) {
	requireIntegrationAssets(t)
	requireDockerOnWorkers(t)
	setupMonitoringIntegrationBucket(t)

	assert.Equal(t, 0, jobDeploymentSeq(t, monitoringJobNodeExporter))
	assert.Equal(t, 1, jobDeploymentSeq(t, monitoringJobPrometheus))
	assert.NotEmpty(t, jobAssignedPort(t, monitoringJobNodeExporter, "node_exporter_metrics_port"))
	assert.NotEmpty(t, jobAssignedPort(t, monitoringJobPrometheus, "prometheus_http_port"))
	assert.Equal(
		t,
		latestKVValue(t, "maand/bucket", "node_exporter_metrics_port"),
		jobAssignedPort(t, monitoringJobNodeExporter, "node_exporter_metrics_port"),
	)

	require.NoError(t, deploy.Execute(nil, false))
	assert.True(t, jobAllocationHashesPromoted(t, monitoringJobNodeExporter))
	assert.True(t, jobAllocationHashesPromoted(t, monitoringJobPrometheus))

	require.NoError(t, jobcontrol.Execute(monitoringJobNodeExporter, "", "test", false))
	require.NoError(t, jobcontrol.Execute(monitoringJobPrometheus, "", "test", false))

	workerCount := len(workerIPs(t))
	promWorker := workerIPs(t)[0]
	promPort := jobAssignedPort(t, monitoringJobPrometheus, "prometheus_http_port")
	probe := fmt.Sprintf(
		`curl -sf "http://127.0.0.1:%s/api/v1/targets" | python3 -c 'import json,sys; data=json.load(sys.stdin); targets=[t for t in data.get("data",{}).get("activeTargets",[]) if t.get("labels",{}).get("job")=="node_exporter"]; assert len(targets)==%d; assert all(t.get("health")=="up" for t in targets); print("connected")'`,
		promPort, workerCount,
	)
	out := remoteShellOutput(t, promWorker, probe)
	assert.Contains(t, out, "connected")
}
