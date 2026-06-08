// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package initialize creates or upgrades a maand bucket in the current directory.
package initialize

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/kv"

	"github.com/google/uuid"
)

// Execute initializes a new bucket or upgrades an existing one to the latest schema.
func Execute() error {
	if err := ensureBucketDirectories(); err != nil {
		return err
	}

	isNewDatabase := !data.DatabaseExists()

	db, err := data.OpenDatabase(false)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := data.MigrateSchema(tx); err != nil {
		return err
	}

	bucketInitialized, err := data.BucketInitialized(tx)
	if err != nil {
		return err
	}

	var bucketID string
	if isNewDatabase || !bucketInitialized {
		bucketID = uuid.NewString()
		if err := data.InsertBucketRecord(tx, bucketID); err != nil {
			return err
		}
		if err := ensureDefaultMaandConfig(); err != nil {
			return err
		}
	} else {
		bucketID, err = data.GetBucketID(tx)
		if err != nil {
			return err
		}
	}

	if err := ensureWorkspaceFiles(); err != nil {
		return err
	}
	if err := ensureCA(bucketID); err != nil {
		return err
	}
	if err := kv.EnsureEncryptionKey(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	if !bucket.QuietCLIOutput() {
		if isNewDatabase || !bucketInitialized {
			fmt.Println("maand bucket initialized")
		} else {
			fmt.Printf("maand bucket upgraded (schema version %d)\n", data.LatestSchemaVersion)
		}
	}
	return nil
}

func ensureBucketDirectories() error {
	dirs := []string{
		path.Join(bucket.Location, "data"),
		path.Join(bucket.Location, "workspace", "jobs"),
		path.Join(bucket.Location, "logs"),
		path.Join(bucket.Location, "tmp"),
		path.Join(bucket.SecretLocation),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}

func ensureWorkspaceFiles() error {
	workersJSON := path.Join(bucket.WorkspaceLocation, "workers.json")
	if _, err := os.Stat(workersJSON); os.IsNotExist(err) {
		if err := os.WriteFile(workersJSON, []byte("[]"), 0o644); err != nil {
			return fmt.Errorf("create workers.json: %w", err)
		}
	}

	bucketConf := path.Join(bucket.WorkspaceLocation, "bucket.conf")
	if _, err := os.Stat(bucketConf); os.IsNotExist(err) {
		if err := os.WriteFile(bucketConf, []byte(bucket.DefaultBucketConf()), 0o644); err != nil {
			return fmt.Errorf("create bucket.conf: %w", err)
		}
	}
	return nil
}

func ensureDefaultMaandConfig() error {
	conf := bucket.MaandConf{
		UseSUDO:    true,
		SSHUser:    "agent",
		SSHKeyFile: "worker.key",
		CertsTTL:   60,
	}
	if err := bucket.WriteMaandConf(&conf); err != nil {
		return fmt.Errorf("write maand.conf: %w", err)
	}
	return nil
}

func ensureCA(bucketID string) error {
	caCertPath := path.Join(bucket.SecretLocation, "ca.crt")
	caKeyPath := path.Join(bucket.SecretLocation, "ca.key")
	_, certErr := os.Stat(caCertPath)
	_, keyErr := os.Stat(caKeyPath)
	certMissing := os.IsNotExist(certErr)
	keyMissing := os.IsNotExist(keyErr)
	if certMissing && keyMissing {
		return generateCA(bucket.SecretLocation, pkix.Name{CommonName: bucketID}, 10*365)
	}
	if certMissing || keyMissing {
		return fmt.Errorf("bucket CA is incomplete: secrets must contain both ca.crt and ca.key")
	}
	if certErr != nil {
		return fmt.Errorf("stat ca.crt: %w", certErr)
	}
	if keyErr != nil {
		return fmt.Errorf("stat ca.key: %w", keyErr)
	}
	return nil
}

func generateCA(secretsDir string, subject pkix.Name, ttlDays int) error {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		NotBefore:             now,
		NotAfter:              now.Add(time.Duration(ttlDays) * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	keyBits := 4096
	if bucket.TestMode() {
		keyBits = 1024
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	certFile, err := os.Create(path.Join(secretsDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("create ca.crt: %w", err)
	}
	defer func() {
		_ = certFile.Close()
	}()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return err
	}

	keyFile, err := os.Create(path.Join(secretsDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("create ca.key: %w", err)
	}
	defer func() {
		_ = keyFile.Close()
	}()

	return pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
}
