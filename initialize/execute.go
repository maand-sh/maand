// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package initialize

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"fmt"
	"github.com/google/uuid"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"math/big"
	"os"
	"path"
	"time"
)

//go:embed Dockerfile
var dockerFile []byte

var BucketAlreadyInitializedErr = fmt.Errorf("maand is already initialized in this directory")

func Execute() error {
	var dbFile = path.Join(bucket.Location, "data/maand.db")
	if f, err := os.Stat(dbFile); f != nil || os.IsExist(err) {
		return BucketAlreadyInitializedErr
	}

	err := os.MkdirAll(path.Join(bucket.Location, "data"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create data directory: %w", err)
	}

	err = os.MkdirAll(path.Join(bucket.Location, "workspace", "jobs"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create workspace jobs directory: %w", err)
	}

	err = os.MkdirAll(path.Join(bucket.Location, "logs"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create logs directory: %w", err)
	}

	err = os.MkdirAll(path.Join(bucket.Location, "secrets"), os.ModePerm)
	if err != nil {
		return fmt.Errorf("unable to create secrets directory: %w", err)
	}

	db, err := data.GetDatabase(false)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	err = data.SetupMaand(tx)
	if err != nil {
		return err
	}

	bucketId := uuid.NewString()
	_, err = tx.Exec("INSERT INTO bucket (bucket_id, update_seq) VALUES (?, ?)", bucketId, 0)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	workersJson := path.Join(bucket.WorkspaceLocation, "workers.json")
	if _, err := os.Stat(workersJson); os.IsNotExist(err) {
		err = os.WriteFile(workersJson, []byte("[]"), os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create workers.json, %w", err)
		}
	}

	conf := utils.MaandConf{
		UseSUDO:    true,
		SSHUser:    "agent",
		SSHKeyFile: "worker.key",
		CertsTTL:   60,
	}

	err = utils.WriteMaandConf(&conf)
	if err != nil {
		return fmt.Errorf("unable to write maand.conf, %w", err)
	}

	bucketConf := path.Join(bucket.WorkspaceLocation, "bucket.conf")
	if _, err := os.Stat(bucketConf); os.IsNotExist(err) {
		err = os.WriteFile(bucketConf, []byte(""), os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create bucket.conf, %w", err)
		}
	}

	caCrtFile := path.Join(bucket.SecretLocation, "ca.crt")
	caKeyFile := path.Join(bucket.SecretLocation, "ca.key")
	_, caCrtErr := os.Stat(caCrtFile)
	_, caKeyErr := os.Stat(caKeyFile)
	if os.IsNotExist(caCrtErr) && os.IsNotExist(caKeyErr) {
		err = generateCA(bucket.SecretLocation, pkix.Name{CommonName: bucketId}, 10*365)
		if err != nil {
			return err
		}
	}

	dockerFilePath := path.Join(bucket.Location, "Dockerfile")
	if _, err := os.Stat(dockerFilePath); os.IsNotExist(err) {
		err = os.WriteFile(dockerFilePath, dockerFile, os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create docker file, %w", err)
		}
	}

	requirementFilePath := path.Join(bucket.Location, "requirements.txt")
	if _, err := os.Stat(requirementFilePath); os.IsNotExist(err) {
		err = os.WriteFile(requirementFilePath, []byte(""), os.ModePerm)
		if err != nil {
			return fmt.Errorf("unable to create requirements.txt, %w", err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	err = data.UpdateJournalModeDefault(db)
	if err != nil {
		return err
	}

	return utils.ExecuteCommand([]string{"sync"})
}

func generateCA(path string, subject pkix.Name, ttlDays int) error {
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

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// Save CA certificate
	certFile, err := os.Create(fmt.Sprintf("%s/ca.crt", path))
	if err != nil {
		return fmt.Errorf("unable to create ca.crt, %w", err)
	}
	defer func() {
		_ = certFile.Close()
	}()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return err
	}

	// Save CA private key
	keyFile, err := os.Create(fmt.Sprintf("%s/ca.key", path))
	if err != nil {
		return fmt.Errorf("unable to create ca.key, %w", err)
	}
	defer func() {
		_ = keyFile.Close()
	}()

	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		return err
	}

	return nil
}
