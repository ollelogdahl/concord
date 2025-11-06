package unit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type NodeCertFiles struct {
	CertPath string
	KeyPath  string
}

func GenerateMultiNodeCerts(numNodes int) (string, string, []NodeCertFiles, error) {
	tempDir, err := os.MkdirTemp("", "concord-test-certs-*")
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// 1. Create CA Certificate and Key
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization: []string{"Concord Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1 Year
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCertPath := filepath.Join(tempDir, "test_ca.pem")
	if err := savePEM(caCertPath, "CERTIFICATE", caBytes); err != nil {
		return "", "", nil, err
	}

	// 2. Create N Node Certificates
	nodeFiles := make([]NodeCertFiles, numNodes)

	for i := 0; i < numNodes; i++ {
		nodeCert := &x509.Certificate{
			SerialNumber: big.NewInt(int64(i + 1)),
			Subject: pkix.Name{
				Organization: []string{fmt.Sprintf("Concord Test Node %d", i)},
			},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
			DNSNames:    []string{"localhost"},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().AddDate(1, 0, 0),
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		}

		nodeKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to generate node %d key: %w", i, err)
		}

		nodeCertBytes, err := x509.CreateCertificate(rand.Reader, nodeCert, ca, &nodeKey.PublicKey, caKey)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to create node %d certificate: %w", i, err)
		}

		nodeCertPath := filepath.Join(tempDir, fmt.Sprintf("node_%d.pem", i))
		nodeKeyPath := filepath.Join(tempDir, fmt.Sprintf("node_%d_key.pem", i))

		if err := savePEM(nodeCertPath, "CERTIFICATE", nodeCertBytes); err != nil {
			return "", "", nil, err
		}
		if err := savePEM(nodeKeyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(nodeKey)); err != nil {
			return "", "", nil, err
		}

		nodeFiles[i] = NodeCertFiles{
			CertPath: nodeCertPath,
			KeyPath:  nodeKeyPath,
		}
	}

	return tempDir, caCertPath, nodeFiles, nil
}

// savePEM writes PEM-encoded data to a file.
func savePEM(path, pemType string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	defer file.Close()

	if err := pem.Encode(file, &pem.Block{Type: pemType, Bytes: data}); err != nil {
		return fmt.Errorf("failed to encode PEM to file %s: %w", path, err)
	}
	return nil
}

func CleanupTestCerts(dir string) {
	os.RemoveAll(dir)
}
