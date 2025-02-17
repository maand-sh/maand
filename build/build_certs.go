package build

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"fmt"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"math/big"
	"net"
	"os"
	"path"
	"strings"
	"time"
)

func Certs(tx *sql.Tx) {
	caHash, err := utils.CalculateFileMD5(path.Join(bucket.SecretLocation, "ca.crt"))
	utils.Check(err)

	utils.UpdateHash(tx, "build", "ca", caHash)

	workers := data.GetWorkers(tx, nil)

	for _, workerIP := range workers {
		workerDirPath := path.Join(bucket.TempLocation, "workers", workerIP)

		jobs := data.GetAllocatedJobs(tx, workerIP)
		for _, job := range jobs {
			rows, err := tx.Query("SELECT name FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)", job)
			utils.Check(err)

			ns := fmt.Sprintf("maand/job/%s/worker/%s", job, workerIP)

			certsMap := map[string]string{}
			for rows.Next() {
				jobDir := path.Join(workerDirPath, "jobs", job)
				err = os.MkdirAll(path.Join(jobDir, "certs"), os.ModePerm)
				utils.Check(err)

				updateCerts := utils.HashChanged(tx, "build", "ca")

				var certName string
				err = rows.Scan(&certName)
				utils.Check(err)

				certName = fmt.Sprintf("certs/%s", certName)
				certPath := path.Join(jobDir, certName)

				vKey, err := utils.GetKVStore().Get(tx, ns, certName+".key")
				if err != nil && !strings.Contains(err.Error(), "not found") {
					utils.Check(err)
				}
				if len(vKey) > 0 {
					err = os.WriteFile(certPath+".key", []byte(vKey), os.ModePerm)
					utils.Check(err)
				}

				vCrt, err := utils.GetKVStore().Get(tx, ns, certName+".crt")
				if err != nil && !strings.Contains(err.Error(), "not found") {
					utils.Check(err)
				}
				if len(vCrt) > 0 {
					err = os.WriteFile(certPath+".crt", []byte(vCrt), os.ModePerm)
					utils.Check(err)

					maandConf := utils.GetMaandConf()
					if !updateCerts && IsCertExpiringSoon(certPath+".crt", maandConf.CertsRenewalBuffer) {
						updateCerts = true
					}
				}
				if !updateCerts && utils.HashChanged(tx, "build_certs", job) {
					updateCerts = true
				}

				if updateCerts {
					maandConf := utils.GetMaandConf()
					GenerateCert(jobDir, certName, pkix.Name{CommonName: "maand"}, workerIP, maandConf.CertsTTL)
				}

				certPub, err := os.ReadFile(certPath + ".crt")
				utils.Check(err)
				certPri, err := os.ReadFile(certPath + ".key")
				utils.Check(err)

				certsMap[certName+".crt"] = string(certPub)
				certsMap[certName+".key"] = string(certPri)
			}

			err = storeKeyValues(tx, ns, certsMap)
			utils.Check(err)
		}
	}

	// update moved after completion of all allocations
	for _, workerIP := range workers {
		jobs := data.GetAllocatedJobs(tx, workerIP)
		for _, job := range jobs {
			utils.PromoteHash(tx, "build_certs", job)
		}
	}

	utils.PromoteHash(tx, "build", "ca")
}

func GenerateCert(path, name string, subject pkix.Name, ipAddress string, ttlDays int) {
	// Load CA certificate
	caCertPEM, err := os.ReadFile(fmt.Sprintf("%s/ca.crt", bucket.SecretLocation))
	utils.Check(err)

	caKeyPEM, err := os.ReadFile(fmt.Sprintf("%s/ca.key", bucket.SecretLocation))
	utils.Check(err)

	// Decode CA certificate
	caCertBlock, _ := pem.Decode(caCertPEM)
	utils.Check(err)

	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	utils.Check(err)

	// Decode CA private key
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	utils.Check(err)

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	utils.Check(err)

	// Generate new certificate
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	utils.Check(err)

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
	utils.Check(err)

	// Sign certificate with CA
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	utils.Check(err)

	// Save certificate
	certFile, err := os.Create(fmt.Sprintf("%s/%s.crt", path, name))
	utils.Check(err)

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	utils.Check(err)

	// Save private key
	keyFile, err := os.Create(fmt.Sprintf("%s/%s.key", path, name))
	utils.Check(err)

	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	utils.Check(err)

	err = certFile.Close()
	utils.Check(err)

	err = keyFile.Close()
	utils.Check(err)
}

func IsCertExpiringSoon(certPath string, days int) bool {
	certPEM, err := os.ReadFile(certPath)
	utils.Check(err)

	block, _ := pem.Decode(certPEM)
	utils.Check(err)

	cert, err := x509.ParseCertificate(block.Bytes)
	utils.Check(err)

	expiryDate := cert.NotAfter.UTC().Add(time.Duration(days) * 24 * time.Hour)
	checkDate := time.Now().UTC()

	return expiryDate.Before(checkDate)
}
