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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path"
	"time"

	"maand/bucket"
	"maand/data"
	"maand/kv"
	"maand/utils"
	"maand/workspace"
)

const (
	certPublicFilePerm  = 0o644
	certPrivateFilePerm = 0o600
)

type jobCertSpec struct {
	name    string
	pkcs8   bool
	one     bool
	subject pkix.Name
}

func BuildCerts(tx *sql.Tx) error {
	caHash, err := utils.CalculateFileMD5(path.Join(bucket.SecretLocation, "ca.crt"))
	if err != nil {
		return err
	}

	if err := data.UpdateHash(tx, "build", "ca", caHash); err != nil {
		return err
	}

	jobs, err := data.GetAllAllocatedJobs(tx)
	if err != nil {
		return err
	}

	caChanged, err := data.HashChanged(tx, "build", "ca")
	if err != nil {
		return err
	}

	for _, jobName := range jobs {
		if err := buildJobCerts(tx, jobName, caChanged); err != nil {
			return err
		}
	}

	return data.PromoteHash(tx, "build", "ca")
}

func buildJobCerts(tx *sql.Tx, jobName string, caChanged bool) error {
	jobDir := path.Join("jobs", jobName)

	jobCertConfigChanged, err := data.HashChanged(tx, "build_certs", jobName)
	if err != nil {
		return err
	}
	jobRegenerate := caChanged || jobCertConfigChanged

	workerIPs, err := data.GetNonRemovedAllocations(tx, jobName)
	if err != nil {
		return err
	}

	certSpecs, err := loadJobCertSpecs(tx, jobName)
	if err != nil {
		return err
	}

	if len(certSpecs) == 0 {
		allocatedWorkers, err := data.GetAllocatedWorkers(tx, jobName)
		if err != nil {
			return err
		}
		if err := purgeJobAllocationCertKV(tx, jobName, allocatedWorkers); err != nil {
			return err
		}
		return data.PromoteHash(tx, "build_certs", jobName)
	}

	if len(workerIPs) == 0 {
		allocatedWorkers, err := data.GetAllocatedWorkers(tx, jobName)
		if err != nil {
			return err
		}
		if err := purgeJobAllocationCertKV(tx, jobName, allocatedWorkers); err != nil {
			return err
		}
		return data.PromoteHash(tx, "build_certs", jobName)
	}

	maandConf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}

	certVariablesByNamespace := map[string]map[string]string{}

	for _, spec := range certSpecs {
		certKeyPrefix := fmt.Sprintf("certs/%s", spec.name)
		regenerate, err := certNeedsRegeneration(
			jobName,
			spec.name,
			jobRegenerate,
			workerIPs,
			maandConf.CertsRenewalBuffer,
		)
		if err != nil {
			return err
		}

		if spec.one {
			certPEM, keyPEM, err := buildSharedCertPEM(
				jobName,
				jobDir,
				spec,
				workerIPs,
				regenerate,
				maandConf.CertsTTL,
			)
			if err != nil {
				return err
			}
			for _, workerIP := range workerIPs {
				if err := addCertVariables(
					certVariablesByNamespace,
					jobName,
					workerIP,
					certKeyPrefix,
					certPEM,
					keyPEM,
				); err != nil {
					return err
				}
			}
			continue
		}

		for _, workerIP := range workerIPs {
			workerRegenerate := regenerate
			if !workerRegenerate {
				needs, err := workerCertNeedsRegeneration(
					jobName,
					workerIP,
					spec.name,
					maandConf.CertsRenewalBuffer,
				)
				if err != nil {
					return err
				}
				workerRegenerate = needs
			}

			certPEM, keyPEM, err := buildWorkerCertPEM(
				jobName,
				jobDir,
				workerIP,
				spec,
				workerRegenerate,
				maandConf.CertsTTL,
			)
			if err != nil {
				return err
			}
			if err := addCertVariables(
				certVariablesByNamespace,
				jobName,
				workerIP,
				certKeyPrefix,
				certPEM,
				keyPEM,
			); err != nil {
				return err
			}
		}
	}

	for namespace, certVariables := range certVariablesByNamespace {
		if err := syncCertKeyValues(namespace, certVariables); err != nil {
			return err
		}
	}

	return data.PromoteHash(tx, "build_certs", jobName)
}

func loadJobCertSpecs(tx *sql.Tx, jobName string) ([]jobCertSpec, error) {
	rows, err := tx.Query(
		"SELECT name, pkcs8, one, subject FROM job_certs WHERE job_id = (SELECT job_id FROM job WHERE name = ?)",
		jobName,
	)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	specs := make([]jobCertSpec, 0)
	for rows.Next() {
		var certName, subject string
		var pkcs8, one int

		if err := rows.Scan(&certName, &pkcs8, &one, &subject); err != nil {
			return nil, bucket.DatabaseError(err)
		}

		var jobSubject workspace.CertSubject
		if err := json.Unmarshal([]byte(subject), &jobSubject); err != nil {
			return nil, fmt.Errorf("%w: job %s cert %s %w", bucket.ErrInvalidManifest, jobName, certName, err)
		}

		specs = append(specs, jobCertSpec{
			name:    certName,
			pkcs8:   pkcs8 == 1,
			one:     one == 1,
			subject: pkix.Name{CommonName: jobSubject.CommonName},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, bucket.DatabaseError(err)
	}
	return specs, nil
}

func purgeJobAllocationCertKV(tx *sql.Tx, jobName string, workerIPs []string) error {
	for _, workerIP := range workerIPs {
		namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
		if err := syncCertKeyValues(namespace, map[string]string{}); err != nil {
			return err
		}
	}
	return nil
}

func certNeedsRegeneration(
	jobName, certName string,
	jobRegenerate bool,
	workerIPs []string,
	renewalBufferDays int,
) (bool, error) {
	if jobRegenerate {
		return true, nil
	}
	for _, workerIP := range workerIPs {
		needs, err := workerCertNeedsRegeneration(jobName, workerIP, certName, renewalBufferDays)
		if err != nil {
			return false, err
		}
		if needs {
			return true, nil
		}
	}
	return false, nil
}

func workerCertNeedsRegeneration(
	jobName, workerIP, certName string,
	renewalBufferDays int,
) (bool, error) {
	namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
	certKeyPrefix := fmt.Sprintf("certs/%s", certName)

	storedKey, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".key")
	if err != nil && !errors.Is(err, kv.ErrNotFound) {
		return false, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	if len(storedKey.Value) == 0 {
		return true, nil
	}

	storedCert, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".crt")
	if err != nil && !errors.Is(err, kv.ErrNotFound) {
		return false, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	if len(storedCert.Value) == 0 {
		return true, nil
	}

	return certPEMExpiringSoon([]byte(storedCert.Value), renewalBufferDays)
}

func certPEMExpiringSoon(certPEM []byte, renewalBufferDays int) (bool, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return true, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	return CertNeedsRenewal(cert.NotAfter, renewalBufferDays, time.Now().UTC()), nil
}

func buildSharedCertPEM(
	jobName, jobDir string,
	spec jobCertSpec,
	workerIPs []string,
	regenerate bool,
	ttlDays int,
) ([]byte, []byte, error) {
	if regenerate {
		firstWorker := workerIPs[0]
		certPath := workerCertDir(firstWorker, jobDir)
		if err := os.MkdirAll(certPath, 0o755); err != nil {
			return nil, nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
		}
		if err := GenerateCert(certPath, spec.name, spec.subject, workerIPs, ttlDays, spec.pkcs8); err != nil {
			return nil, nil, err
		}
		certPEM, keyPEM, err := readCertFiles(certPath, spec.name)
		if err != nil {
			return nil, nil, err
		}
		for _, workerIP := range workerIPs {
			targetPath := workerCertDir(workerIP, jobDir)
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return nil, nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
			}
			if err := writeCertFiles(targetPath, spec.name, certPEM, keyPEM); err != nil {
				return nil, nil, err
			}
		}
		return certPEM, keyPEM, nil
	}

	namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIPs[0])
	certKeyPrefix := fmt.Sprintf("certs/%s", spec.name)
	certEntry, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".crt")
	if err != nil {
		return nil, nil, err
	}
	keyEntry, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".key")
	if err != nil {
		return nil, nil, err
	}
	return []byte(certEntry.Value), []byte(keyEntry.Value), nil
}

func buildWorkerCertPEM(
	jobName, jobDir, workerIP string,
	spec jobCertSpec,
	regenerate bool,
	ttlDays int,
) ([]byte, []byte, error) {
	certPath := workerCertDir(workerIP, jobDir)
	if err := os.MkdirAll(certPath, 0o755); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	if regenerate {
		if err := GenerateCert(certPath, spec.name, spec.subject, []string{workerIP}, ttlDays, spec.pkcs8); err != nil {
			return nil, nil, err
		}
		return readCertFiles(certPath, spec.name)
	}

	namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
	certKeyPrefix := fmt.Sprintf("certs/%s", spec.name)
	certEntry, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".crt")
	if err != nil {
		return nil, nil, err
	}
	keyEntry, err := kv.GetKVStore().Get(namespace, certKeyPrefix+".key")
	if err != nil {
		return nil, nil, err
	}
	return []byte(certEntry.Value), []byte(keyEntry.Value), nil
}

func addCertVariables(
	certVariablesByNamespace map[string]map[string]string,
	jobName, workerIP, certKeyPrefix string,
	certPEM, keyPEM []byte,
) error {
	namespace := fmt.Sprintf("maand/job/%s/worker/%s", jobName, workerIP)
	if certVariablesByNamespace[namespace] == nil {
		certVariablesByNamespace[namespace] = map[string]string{}
	}
	certVariablesByNamespace[namespace][certKeyPrefix+".crt"] = string(certPEM)
	certVariablesByNamespace[namespace][certKeyPrefix+".key"] = string(keyPEM)
	return nil
}

func workerCertDir(workerIP, jobDir string) string {
	return path.Join(bucket.TempLocation, "workers", workerIP, jobDir, "certs")
}

func readCertFiles(certPath, certName string) ([]byte, []byte, error) {
	certPEM, err := os.ReadFile(path.Join(certPath, certName+".crt"))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	keyPEM, err := os.ReadFile(path.Join(certPath, certName+".key"))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	return certPEM, keyPEM, nil
}

func writeCertFiles(certPath, certName string, certPEM, keyPEM []byte) error {
	if err := os.WriteFile(path.Join(certPath, certName+".crt"), certPEM, certPublicFilePerm); err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	if err := os.WriteFile(path.Join(certPath, certName+".key"), keyPEM, certPrivateFilePerm); err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	return nil
}

func GenerateCert(certPath, name string, subject pkix.Name, ipAddresses []string, ttlDays int, usePKCS8 bool) error {
	caCertPEM, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.crt"))
	if err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	caKeyPEM, err := os.ReadFile(path.Join(bucket.SecretLocation, "ca.key"))
	if err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("%w: decoding ca.crt: no PEM block found", bucket.ErrUnexpectedError)
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("%w: decoding ca.crt %w", bucket.ErrUnexpectedError, err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("%w: decoding ca.key: no PEM block found", bucket.ErrUnexpectedError)
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("%w: decoding ca.key %w", bucket.ErrUnexpectedError, err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("%w: serial number %w", bucket.ErrUnexpectedError, err)
	}

	ipList := []net.IP{net.ParseIP("127.0.0.1")}
	for _, ip := range ipAddresses {
		ipList = append(ipList, net.ParseIP(ip))
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		IPAddresses:           ipList,
		NotBefore:             now,
		NotAfter:              now.Add(time.Duration(ttlDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("%w: private key %w", bucket.ErrUnexpectedError, err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("%w: sign cert with ca %w", bucket.ErrUnexpectedError, err)
	}

	certFile, err := os.OpenFile(path.Join(certPath, name+".crt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, certPublicFilePerm)
	if err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	defer func() {
		_ = certFile.Close()
	}()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return fmt.Errorf("%w: encoding certificate %w", bucket.ErrUnexpectedError, err)
	}

	keyFile, err := os.OpenFile(path.Join(certPath, name+".key"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, certPrivateFilePerm)
	if err != nil {
		return fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	defer func() {
		_ = keyFile.Close()
	}()

	keyBlock, err := marshalPrivateKey(privateKey, usePKCS8)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyFile, keyBlock); err != nil {
		return fmt.Errorf("%w: encoding private key %w", bucket.ErrUnexpectedError, err)
	}

	return nil
}

func marshalPrivateKey(privateKey *rsa.PrivateKey, usePKCS8 bool) (*pem.Block, error) {
	if usePKCS8 {
		keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("%w: marshal pkcs8 key %w", bucket.ErrUnexpectedError, err)
		}
		return &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}, nil
	}
	return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}, nil
}

func IsCertExpiringSoon(certPath string, days int) (bool, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false, fmt.Errorf("%w: %w", bucket.ErrUnexpectedError, err)
	}
	return certPEMExpiringSoon(certPEM, days)
}
