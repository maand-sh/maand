// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"crypto/x509/pkix"
	"encoding/pem"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/initialize"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBuildCertSecrets(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	bucket.Location = dir
	bucket.UpdatePath()
	require.NoError(t, os.MkdirAll(bucket.WorkspaceLocation, 0o755))
	require.NoError(t, os.WriteFile(path.Join(bucket.WorkspaceLocation, "workers.json"), []byte("[]"), 0o644))
	require.NoError(t, initialize.Execute())
}

func TestGenerateCert_pkcs8(t *testing.T) {
	setupBuildCertSecrets(t)

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 30, true))

	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	assert.Equal(t, "PRIVATE KEY", block.Type)
}

func TestGenerateCert_pkcs1(t *testing.T) {
	setupBuildCertSecrets(t)

	certDir := t.TempDir()
	require.NoError(t, GenerateCert(certDir, "tls", pkix.Name{CommonName: "test"}, []string{"10.0.0.1"}, 30, false))

	keyPEM, err := os.ReadFile(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	assert.Equal(t, "RSA PRIVATE KEY", block.Type)

	info, err := os.Stat(path.Join(certDir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
