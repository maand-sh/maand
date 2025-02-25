// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"math/big"
	"net"
	"os"
	"path"
	"time"
)

func Certs(tx *sql.Tx) error {
	caHash, err := utils.CalculateFileMD5(path.Join(bucket.SecretLocation, "ca.crt"))
	if err != nil {
		return err
	}

	err = data.UpdateHash(tx, "build", "ca", caHash)
	if err != nil {
		return err
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	for _, workerIP := range workers {
		workerDirPath := path.Join(bucket.TempLocation, "workers", workerIP)

		jobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		for _, job := range jobs {
			rows, err := tx.Query("SELECT name FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
			if err != nil {
				return err
			}

			ns := fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP)

			certsMap := map[string]string{}
			for rows.Next() {
				jobDir := path.Join(workerDirPath, "jobs", job)
				err = os.MkdirAll(path.Join(jobDir, "certs"), os.ModePerm)
				if err != nil {
					return err
				}

				updateCerts, err := data.HashChanged(tx, "build", "ca")
				if err != nil {
					return err
				}

				var certName string
				err = rows.Scan(&certName)
				if err != nil {
					return err
				}

				certName = fmt.Sprintf("certs/%s", certName)
				certPath := path.Join(jobDir, certName)

				vKey, err := kv.GetKVStore().Get(tx, ns, certName+".key")
				var errNotFound = kv.NewNotFoundError(ns, certName+".key")
				if err != nil && !errors.As(err, &errNotFound) {
					return err
				}
				if len(vKey) > 0 {
					err = os.WriteFile(certPath+".key", []byte(vKey), os.ModePerm)
					if err != nil {
						return err
					}
				}

				vCrt, err := kv.GetKVStore().Get(tx, ns, certName+".crt")
				errNotFound = kv.NewNotFoundError(ns, certName+".crt")
				if err != nil && !errors.As(err, &errNotFound) {
					return err
				}
				if len(vCrt) > 0 {
					err = os.WriteFile(certPath+".crt", []byte(vCrt), os.ModePerm)
					if err != nil {
						return err
					}

					maandConf, err := utils.GetMaandConf()
					if err != nil {
						return err
					}

					certExpired, err := IsCertExpiringSoon(certPath+".crt", maandConf.CertsRenewalBuffer)
					if err != nil {
						return err
					}

					if !updateCerts && certExpired {
						updateCerts = true
					}
				}

				jobHashChanged, err := data.HashChanged(tx, "build_certs", job)
				if err != nil {
					return err
				}

				if !updateCerts && jobHashChanged {
					updateCerts = true
				}

				if updateCerts {
					maandConf, err := utils.GetMaandConf()
					if err != nil {
						return err
					}

					err = GenerateCert(jobDir, certName, pkix.Name{CommonName: "maand"}, workerIP, maandConf.CertsTTL)
					if err != nil {
						return err
					}
				}

				certPub, err := os.ReadFile(certPath + ".crt")
				if err != nil {
					return err
				}
				certPri, err := os.ReadFile(certPath + ".key")
				if err != nil {
					return err
				}

				certsMap[certName+".crt"] = string(certPub)
				certsMap[certName+".key"] = string(certPri)
			}

			err = storeKeyValues(tx, ns, certsMap)
			if err != nil {
				return err
			}
		}
	}

	// update moved after completion of all allocations
	for _, workerIP := range workers {
		jobs, err := data.GetAllocatedJobs(tx, workerIP)
		if err != nil {
			return err
		}

		for _, job := range jobs {
			err = data.PromoteHash(tx, "build_certs", job)
			if err != nil {
				return err
			}
		}
	}

	err = data.PromoteHash(tx, "build", "ca")
	if err != nil {
		return err
	}
	return nil
}

func GenerateCert(path, name string, subject pkix.Name, ipAddress string, ttlDays int) error {
	// Load CA certificate
	caCertPEM, err := os.ReadFile(fmt.Sprintf("%s/ca.crt", bucket.SecretLocation))
	if err != nil {
		return err
	}

	caKeyPEM, err := os.ReadFile(fmt.Sprintf("%s/ca.key", bucket.SecretLocation))
	if err != nil {
		return err
	}

	// Decode CA certificate
	caCertBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return err
	}

	// Decode CA private key
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return err
	}

	// Generate new certificate
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP(ipAddress)},
		NotBefore:             now,
		NotAfter:              now.Add(time.Duration(ttlDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// Sign certificate with CA
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	// Save certificate
	certFile, err := os.Create(fmt.Sprintf("%s/%s.crt", path, name))
	if err != nil {
		return err
	}

	defer func() {
		_ = certFile.Close()
	}()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return err
	}

	// Save private key
	keyFile, err := os.Create(fmt.Sprintf("%s/%s.key", path, name))
	if err != nil {
		return err
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

func IsCertExpiringSoon(certPath string, days int) (bool, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false, err
	}

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, err
	}

	expiryDate := cert.NotAfter.UTC().Add(time.Duration(days) * 24 * time.Hour)
	checkDate := time.Now().UTC()

	return expiryDate.Before(checkDate), nil
}
