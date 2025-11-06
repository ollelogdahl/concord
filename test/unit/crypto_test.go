package unit

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/ollelogdahl/concord"
	"github.com/stretchr/testify/require"
)

func setupConcord(t *testing.T, name, bindAddr, advAddr string, ca string, certs *NodeCertFiles) *concord.Concord {
	config := concord.Config{
		Name:       name,
		BindAddr:   bindAddr,
		AdvAddr:    advAddr,
		LogHandler: slog.NewTextHandler(os.Stdout, nil),
	}

	if certs != nil {
		config.TLSCertFile = certs.CertPath
		config.TLSKeyFile = certs.KeyPath
		config.TLSCAFile = ca
	}

	c := concord.New(config)
	require.NoError(t, c.Start(), "Concord instance failed to start")
	return c
}

func TestConcordSecure(t *testing.T) {
	certDir, ca, nodeFiles, err := GenerateMultiNodeCerts(2)
	require.NoError(t, err)
	defer CleanupTestCerts(certDir)

	nodeA := setupConcord(t, "NodeA", ":12000", "localhost:12000", ca, &nodeFiles[0])
	defer nodeA.Stop()
	nodeB := setupConcord(t, "NodeB", ":12001", "localhost:12001", ca, &nodeFiles[0])
	defer nodeB.Stop()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = nodeA.Create()
	require.NoError(t, err)

	err = nodeB.Join(ctxWithTimeout, "localhost:12000")
	require.NoError(t, err)
}

func TestSecureInsecureCannotJoin(t *testing.T) {
	certDir, ca, nodeFiles, err := GenerateMultiNodeCerts(1)
	require.NoError(t, err)
	defer CleanupTestCerts(certDir)

	nodeA := setupConcord(t, "NodeA", ":12000", "localhost:12000", ca, &nodeFiles[0])
	defer nodeA.Stop()

	nodeB := setupConcord(t, "NodeB", ":12001", "localhost:12001", "", &NodeCertFiles{
		CertPath: "",
		KeyPath:  "",
	})
	defer nodeB.Stop()

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = nodeA.Create()
	require.NoError(t, err)

	err = nodeB.Join(ctxWithTimeout, "localhost:12000")
	require.Error(t, err)
}
