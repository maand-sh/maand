package initialize

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"errors"
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

func Execute() {
	var dbFile = path.Join(bucket.Location, "data/maand.db")
	if f, err := os.Stat(dbFile); f != nil || os.IsExist(err) {
		utils.Check(errors.New("maand is already initialized"))
	}

	err := utils.ExecuteCommand([]string{fmt.Sprintf("mkdir -p %s/{data,workspace,logs,secrets}", bucket.Location), fmt.Sprintf("mkdir -p %s/jobs", bucket.WorkspaceLocation)})
	utils.Check(err)

	db, err := data.GetDatabase(false)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)

	err = data.SetupMaand(tx)
	utils.Check(err)

	bucketId := uuid.NewString()

	_, err = tx.Exec("INSERT INTO bucket (bucket_id, update_seq) VALUES (?, ?)", bucketId, 0)
	utils.Check(err)

	workersJson := path.Join(bucket.WorkspaceLocation, "workers.json")
	if _, err := os.Stat(workersJson); os.IsNotExist(err) {
		err = os.WriteFile(workersJson, []byte("[]"), os.ModePerm)
		utils.Check(err)
	}

	conf := utils.MaandConf{
		UseSUDO:    true,
		SSHUser:    "agent",
		SSHKeyFile: "worker.key",
		CertsTTL:   60,
	}
	utils.WriteMaandConf(&conf)

	bucketConf := path.Join(bucket.WorkspaceLocation, "bucket.conf")
	if _, err := os.Stat(bucketConf); os.IsNotExist(err) {
		err = os.WriteFile(bucketConf, []byte(""), os.ModePerm)
		utils.Check(err)
	}

	caCrtFile := path.Join(bucket.SecretLocation, "ca.crt")
	caKeyFile := path.Join(bucket.SecretLocation, "ca.key")
	_, caCrtErr := os.Stat(caCrtFile)
	_, caKeyErr := os.Stat(caKeyFile)
	if os.IsNotExist(caCrtErr) && os.IsNotExist(caKeyErr) {
		generateCA(bucket.SecretLocation, pkix.Name{CommonName: bucketId}, 10*365)
	}

	dockerFilePath := path.Join(bucket.Location, "Dockerfile")
	if _, err := os.Stat(dockerFilePath); os.IsNotExist(err) {
		_ = os.WriteFile(dockerFilePath, dockerFile, os.ModePerm)
	}

	requirementFilePath := path.Join(bucket.Location, "requirements.txt")
	if _, err := os.Stat(requirementFilePath); os.IsNotExist(err) {
		_ = os.WriteFile(requirementFilePath, []byte(""), os.ModePerm)
	}

	err = tx.Commit()
	utils.Check(err)

	_ = utils.ExecuteCommand([]string{"sync"})
}

func generateCA(path string, subject pkix.Name, ttlDays int) {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	utils.Check(err)

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
	utils.Check(err)

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	utils.Check(err)

	// Save CA certificate
	certFile, err := os.Create(fmt.Sprintf("%s/ca.crt", path))
	utils.Check(err)
	defer certFile.Close()

	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	utils.Check(err)

	// Save CA private key
	keyFile, err := os.Create(fmt.Sprintf("%s/ca.key", path))
	utils.Check(err)
	defer keyFile.Close()

	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	utils.Check(err)
}
