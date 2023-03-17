package oracle

import (
	"bytes"
	"crypto/x509"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"
	"github.com/pavlo-v-chernykh/keystore-go/v4"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// GenerateClientConfiguration function generates following Oracle Client configuration:
// wallet.jks   - Java Wallet format used by JDBC Drivers.
// sqlnet.ora   - Generic Oracle Client Configuration File allowing to specify Wallet Location.
// tnsnames.ora - Oracle Net Service mapped to connections descriptors.
func GenerateClientConfiguration(key *client.Key, db tlsca.RouteToDatabase, profile *client.ProfileStatus) error {
	walletPath := profile.OracleWalletDir(key.ClusterName, db.ServiceName)
	if err := os.MkdirAll(walletPath, 0700); err != nil {
		return trace.Wrap(err)
	}
	password, err := utils.CryptoRandomHex(32)
	if err != nil {
		return trace.Wrap(err)
	}

	localProxyCAPem, err := os.ReadFile(profile.DatabaseLocalCAPath())
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	jksWalletPath, err := createClientWallet(key, localProxyCAPem, password, walletPath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = writeClientConfig(walletPath, jksWalletPath, password)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func createClientWallet(key *client.Key, certPem []byte, password string, walletPath string) (string, error) {
	buff, err := createJKSWallet(key.PrivateKeyPEM(), certPem, certPem, password)
	if err != nil {
		return "", trace.Wrap(err)
	}
	walletFile := filepath.Join(walletPath, "wallet.jks")
	if err := os.WriteFile(walletFile, buff, teleport.FileMaskOwnerOnly); err != nil {
		return "", trace.Wrap(err)
	}
	return walletFile, nil
}

func createJKSWallet(keyPEM, certPEM, caPEM []byte, password string) ([]byte, error) {
	key, err := utils.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ks := keystore.New()
	pkeIn := keystore.PrivateKeyEntry{
		CreationTime: time.Now(),
		PrivateKey:   privateKey,
		CertificateChain: []keystore.Certificate{
			{
				Type:    "x509",
				Content: certPEM,
			},
		},
	}

	if err := ks.SetPrivateKeyEntry("teleportUserCert", pkeIn, []byte(password)); err != nil {
		return nil, trace.Wrap(err)
	}
	trustIn := keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate: keystore.Certificate{
			Type:    "x509",
			Content: caPEM,
		},
	}
	if err := ks.SetTrustedCertificateEntry("teleportLocalCA", trustIn); err != nil {
		return nil, trace.Wrap(err)
	}
	var buff bytes.Buffer
	if err := ks.Store(&buff, []byte(password)); err != nil {
		return nil, trace.Wrap(err)
	}
	return buff.Bytes(), nil
}

func writeClientConfig(path string, jksFile string, password string) error {
	var clientConfiguration = []templateSettings{
		tnsNamesORASettings{
			Host: "localhost",
			// User default values that will be overwritten by JDBC connection string.
			ServiceName: "XE",
			Port:        "2484",
		},
		sqlnetORASettings{
			WalletDir: path,
		},
		jdbcSettings{
			KeyStoreFile:       jksFile,
			TrustStoreFile:     jksFile,
			KeyStorePassword:   password,
			TrustStorePassword: password,
		},
	}

	for _, v := range clientConfiguration {
		if err := writeSettings(v, path); err != nil {
			return trace.Wrap(err, "Failed to write %v", v.configFilename())
		}
	}
	return nil
}
