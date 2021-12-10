package renew

import (
	"bytes"
	"crypto/tls"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

type destinationDir struct {
	dir string
}

func (d *destinationDir) HostID() (string, error) {
	metadata, err := os.ReadFile(filepath.Join(d.dir, MetadataKey))
	if err != nil {
		return "", trace.Wrap(err)
	}
	parts := bytes.Split(metadata, []byte("."))
	if len(parts) != 2 {
		return "", trace.BadParameter("expected host ID and cluster name in metadata")
	}

	return string(parts[0]), nil
}

// TODO: remove me
func (d *destinationDir) TLSConfig() (*tls.Config, error) {
	return nil, nil
	// metadata, err := os.ReadFile(filepath.Join(d.dir, MetadataKey))
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	// parts := bytes.Split(metadata, []byte("."))
	// if len(parts) != 2 {
	// 	return nil, trace.BadParameter("expected host ID and cluster name in metadata")
	// }

	// certBytes, err := os.ReadFile(filepath.Join(d.dir, TLSCertKey))
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	// keyBytes, err := os.ReadFile(filepath.Join(d.dir, PrivateKeyKey))
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }

	// caBytes, err := os.ReadFile(filepath.Join(d.dir, TLSCACertsKey))
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }

	// tlsCert, err := tls.X509KeyPair(certBytes, keyBytes)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }

	// caCerts, err := tlsca.ParseCertificatePEMs(caBytes)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }

	// certPool := x509.NewCertPool()
	// for _, caCert := range caCerts {
	// 	certPool.AddCert(caCert)
	// }

	// tc := utils.TLSConfig(nil)
	// tc.Certificates = []tls.Certificate{tlsCert}
	// tc.RootCAs = certPool
	// tc.ClientCAs = certPool
	// tc.ServerName = auth.EncodeClusterName(string(parts[1]))
	// return tc, nil
}

func (d *destinationDir) Write(name string, data []byte) error {
	if err := os.WriteFile(filepath.Join(d.dir, name), data, 0600); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (d *destinationDir) Read(name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(d.dir, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}
