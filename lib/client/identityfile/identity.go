/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package identityfile handles formatting and parsing of identity files.
package identityfile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/pavlo-v-chernykh/keystore-go/v4"
	"software.sslmate.com/src/go-pkcs12"

	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// Format describes possible file formats how a user identity can be stored.
type Format string

const (
	// FormatFile is when a key + cert are stored concatenated into a single file
	FormatFile Format = "file"

	// FormatOpenSSH is OpenSSH-compatible format, when a key and a cert are stored in
	// two different files (in the same directory)
	FormatOpenSSH Format = "openssh"

	// FormatTLS is a standard TLS format used by common TLS clients (e.g. gRPC) where
	// certificate and key are stored in separate files.
	FormatTLS Format = "tls"

	// FormatKubernetes is a standard Kubernetes format, with all credentials
	// stored in a "kubeconfig" file.
	FormatKubernetes Format = "kubernetes"

	// FormatDatabase produces CA and key pair suitable for configuring a
	// database instance for mutual TLS.
	FormatDatabase Format = "db"

	// FormatWindows produces a certificate suitable for logging
	// in to Windows via Active Directory.
	FormatWindows = "windows"

	// FormatMongo produces CA and key pair in the format suitable for
	// configuring a MongoDB database for mutual TLS authentication.
	FormatMongo Format = "mongodb"

	// FormatCockroach produces CA and key pair in the format suitable for
	// configuring a CockroachDB database for mutual TLS.
	FormatCockroach Format = "cockroachdb"

	// FormatRedis produces CA and key pair in the format suitable for
	// configuring a Redis database for mutual TLS.
	FormatRedis Format = "redis"

	// FormatSnowflake produces public key in the format suitable for
	// configuration Snowflake JWT access.
	FormatSnowflake Format = "snowflake"
	// FormatCassandra produces CA and key pair in the format suitable for
	// configuring a Cassandra database for mutual TLS.
	FormatCassandra Format = "cassandra"
	// FormatScylla produces CA and key pair in the format suitable for
	// configuring a Scylla database for mutual TLS.
	FormatScylla Format = "scylla"

	// FormatElasticsearch produces CA and key pair in the format suitable for
	// configuring Elasticsearch for mutual TLS authentication.
	FormatElasticsearch Format = "elasticsearch"

	// DefaultFormat is what Teleport uses by default
	DefaultFormat = FormatFile

	// FormatOracle produces CA and ke pair in the Oracle wallet format.
	// The execution depend on Orapki binary and if this binary is not found
	// Teleport will print intermediate steps how to convert Teleport certs
	// to Oracle wallet on Oracle Server instance.
	FormatOracle Format = "oracle"
)

// FormatList is a list of all possible FormatList.
type FormatList []Format

// KnownFileFormats is a list of all above formats.
var KnownFileFormats = FormatList{
	FormatFile, FormatOpenSSH, FormatTLS, FormatKubernetes, FormatDatabase, FormatWindows,
	FormatMongo, FormatCockroach, FormatRedis, FormatSnowflake, FormatElasticsearch, FormatCassandra, FormatScylla,
	FormatOracle,
}

// String returns human-readable version of FormatList, ex:
// file, openssh, tls, kubernetes
func (f FormatList) String() string {
	elems := make([]string, len(f))
	for i, format := range f {
		elems[i] = string(format)
	}
	return strings.Join(elems, ", ")
}

// ConfigWriter is a simple filesystem abstraction to allow alternative simple
// read/write for this package.
type ConfigWriter interface {
	// WriteFile writes the given data to path `name`, using the specified
	// permissions if the file is new.
	WriteFile(name string, data []byte, perm os.FileMode) error

	// ReadFile reads the file at tpath `name`
	ReadFile(name string) ([]byte, error)

	// Remove removes a file.
	Remove(name string) error

	// Stat fetches information about a file.
	Stat(name string) (fs.FileInfo, error)
}

// StandardConfigWriter is a trivial ConfigWriter that wraps the relevant `os` functions.
type StandardConfigWriter struct{}

// WriteFile writes data to the named file, creating it if necessary.
func (s *StandardConfigWriter) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// ReadFile reads the file at tpath `name`, returning
func (s *StandardConfigWriter) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// Remove removes the named file or (empty) directory.
// If there is an error, it will be of type *PathError.
func (s *StandardConfigWriter) Remove(name string) error {
	return os.Remove(name)
}

// Stat returns a FileInfo describing the named file.
// If there is an error, it will be of type *PathError.
func (s *StandardConfigWriter) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// WriteConfig holds the necessary information to write an identity file.
type WriteConfig struct {
	// OutputPath is the output path for the identity file. Note that some
	// formats (like FormatOpenSSH and FormatTLS) write multiple output files
	// and use OutputPath as a prefix.
	OutputPath string
	// KeyRing contains the credentials to write to the identity file.
	KeyRing *client.KeyRing
	// WindowsDesktopCerts contains windows desktop certs to write.
	WindowsDesktopCerts map[string][]byte
	// Format is the output format for the identity file.
	Format Format
	// KubeProxyAddr is the public address of the proxy with its kubernetes
	// port. KubeProxyAddr is only used when Format is FormatKubernetes.
	KubeProxyAddr string
	// KubeClusterName is the Kubernetes Cluster name.
	// KubeClusterName is only used when Format is FormatKubernetes.
	KubeClusterName string
	// KubeTLSServerName is the SNI host value passed to the server.
	KubeTLSServerName string
	// KubeStoreAllCAs stores the CAs of all clusters in kubeconfig, instead
	// of just the root cluster's CA.
	KubeStoreAllCAs bool
	// OverwriteDestination forces all existing destination files to be
	// overwritten. When false, user will be prompted for confirmation of
	// overwrite first.
	OverwriteDestination bool
	// Writer is the filesystem implementation.
	Writer ConfigWriter
	// Password is the password for the JKS keystore used by Cassandra format and Oracle wallet.
	Password string
	// AdditionalCACerts contains additional CA certs, used by Cockroach format
	// to distinguish DB Server CA certs from DB Client CA certs.
	AdditionalCACerts [][]byte
}

// Write writes user credentials to disk in a specified format.
// It returns the names of the files successfully written.
func Write(ctx context.Context, cfg WriteConfig) (filesWritten []string, err error) {
	// If no writer was set, use the standard implementation.
	writer := cfg.Writer
	if writer == nil {
		writer = &StandardConfigWriter{}
	}

	if cfg.OutputPath == "" {
		return nil, trace.BadParameter("identity output path is not specified")
	}

	switch cfg.Format {
	// dump user identity into a single file:
	case FormatFile:
		// Identity files only hold a single private key, and all certs are
		// associated with that key. All callers should provide a
		// [client.KeyRing] where [KeyRing.SSHPrivateKey] and
		// [KeyRing.TLSPrivateKey] are equal. Assert that here.
		if !bytes.Equal(cfg.KeyRing.SSHPrivateKey.MarshalSSHPublicKey(), cfg.KeyRing.TLSPrivateKey.MarshalSSHPublicKey()) {
			return nil, trace.BadParameter("identity files don't support mismatched SSH and TLS keys, this is a bug")
		}
		filesWritten = append(filesWritten, cfg.OutputPath)
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		idFile := &identityfile.IdentityFile{
			PrivateKey: cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM(),
			Certs: identityfile.Certs{
				SSH: cfg.KeyRing.Cert,
				TLS: cfg.KeyRing.TLSCert,
			},
		}
		// append trusted host certificate authorities
		for _, ca := range cfg.KeyRing.TrustedCerts {
			// append ssh ca certificates
			for _, publicKey := range ca.AuthorizedKeys {
				knownHost, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
					Hostname:      ca.ClusterName,
					ProxyHost:     cfg.KeyRing.ProxyHost,
					AuthorizedKey: publicKey,
				})
				if err != nil {
					return nil, trace.Wrap(err)
				}
				idFile.CACerts.SSH = append(idFile.CACerts.SSH, []byte(knownHost))
			}
			// append tls ca certificates
			idFile.CACerts.TLS = append(idFile.CACerts.TLS, ca.TLSCertificates...)
		}

		idBytes, err := identityfile.Encode(idFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := writer.WriteFile(cfg.OutputPath, idBytes, identityfile.FilePermissions); err != nil {
			return nil, trace.Wrap(err)
		}

	// dump user identity into separate files:
	case FormatOpenSSH:
		keyPath := cfg.OutputPath
		certPath := keypaths.IdentitySSHCertPath(keyPath)
		filesWritten = append(filesWritten, keyPath, certPath)
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		err = writer.WriteFile(certPath, cfg.KeyRing.Cert, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		sshPrivateKeyPEM, err := cfg.KeyRing.SSHPrivateKey.MarshalSSHPrivateKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = writer.WriteFile(keyPath, sshPrivateKeyPEM, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatWindows:
		for k, cert := range cfg.WindowsDesktopCerts {
			certPath := cfg.OutputPath + "." + k + ".der"
			filesWritten = append(filesWritten, certPath)
			if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, certPath); err != nil {
				return nil, trace.Wrap(err)
			}

			err = writer.WriteFile(certPath, cert, identityfile.FilePermissions)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}

	case FormatCockroach:
		// CockroachDB expects files to be named node.crt, node.key, ca.crt,
		// ca-client.crt
		certPath := filepath.Join(cfg.OutputPath, "node.crt")
		keyPath := filepath.Join(cfg.OutputPath, "node.key")
		casPath := filepath.Join(cfg.OutputPath, "ca.crt")
		clientCAsPath := filepath.Join(cfg.OutputPath, "ca-client.crt")

		filesWritten = append(filesWritten, certPath, keyPath, casPath, clientCAsPath)
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		err = writer.WriteFile(certPath, cfg.KeyRing.TLSCert, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writer.WriteFile(keyPath, cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM(), identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var serverCACerts []byte
		for _, cert := range cfg.AdditionalCACerts {
			serverCACerts = append(serverCACerts, cert...)
		}
		err = writer.WriteFile(casPath, serverCACerts, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var clientCACerts []byte
		for _, ca := range cfg.KeyRing.TrustedCerts {
			for _, cert := range ca.TLSCertificates {
				clientCACerts = append(clientCACerts, cert...)
			}
		}
		err = writer.WriteFile(clientCAsPath, clientCACerts, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	case FormatTLS, FormatDatabase, FormatRedis, FormatElasticsearch, FormatScylla:
		keyPath := cfg.OutputPath + ".key"
		certPath := cfg.OutputPath + ".crt"
		casPath := cfg.OutputPath + ".cas"

		filesWritten = append(filesWritten, keyPath, certPath, casPath)
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}

		err = writer.WriteFile(certPath, cfg.KeyRing.TLSCert, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		err = writer.WriteFile(keyPath, cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM(), identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var caCerts []byte
		for _, ca := range cfg.KeyRing.TrustedCerts {
			for _, cert := range ca.TLSCertificates {
				caCerts = append(caCerts, cert...)
			}
		}
		err = writer.WriteFile(casPath, caCerts, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	// FormatMongo is the same as FormatTLS or FormatDatabase certificate and
	// key are concatenated in the same .crt file which is what Mongo expects.
	case FormatMongo:
		certPath := cfg.OutputPath + ".crt"
		casPath := cfg.OutputPath + ".cas"
		filesWritten = append(filesWritten, certPath, casPath)
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil {
			return nil, trace.Wrap(err)
		}
		err = writer.WriteFile(certPath, append(cfg.KeyRing.TLSCert, cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM()...), identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var caCerts []byte
		for _, ca := range cfg.KeyRing.TrustedCerts {
			for _, cert := range ca.TLSCertificates {
				caCerts = append(caCerts, cert...)
			}
		}
		err = writer.WriteFile(casPath, caCerts, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case FormatSnowflake:
		pubPath := cfg.OutputPath + ".pub"
		filesWritten = append(filesWritten, pubPath)

		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, pubPath); err != nil {
			return nil, trace.Wrap(err)
		}

		var caCerts []byte
		for _, ca := range cfg.KeyRing.TrustedCerts {
			for _, cert := range ca.TLSCertificates {
				block, _ := pem.Decode(cert)
				cert, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				pubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				pubPem := pem.EncodeToMemory(&pem.Block{
					Type:  "PUBLIC KEY",
					Bytes: pubKey,
				})
				caCerts = append(caCerts, pubPem...)
			}
		}

		err = writer.WriteFile(pubPath, caCerts, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case FormatCassandra:
		out, err := writeCassandraFormat(cfg, writer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filesWritten = append(filesWritten, out...)
	case FormatOracle:
		out, err := writeOracleFormat(cfg, writer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filesWritten = append(filesWritten, out...)

	case FormatKubernetes:
		filesWritten = append(filesWritten, cfg.OutputPath)
		// If the user does not want to override, it will merge the previous kubeconfig
		// with the new entry.
		if err := checkOverwrite(ctx, writer, cfg.OverwriteDestination, filesWritten...); err != nil && !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		} else if err == nil {
			// Clean up the existing file, if it exists.
			// This is used when the user wants to overwrite an existing kubeconfig.
			// Without it, kubeconfig.Update would try to parse it and merge in new
			// credentials.
			if err := writer.Remove(cfg.OutputPath); err != nil && !os.IsNotExist(err) {
				return nil, trace.Wrap(err)
			}
		}

		var kubeCluster []string
		if len(cfg.KubeClusterName) > 0 {
			kubeCluster = []string{cfg.KubeClusterName}
		}

		if err := kubeconfig.UpdateConfig(cfg.OutputPath, kubeconfig.Values{
			TeleportClusterName: cfg.KeyRing.ClusterName,
			ClusterAddr:         cfg.KubeProxyAddr,
			Credentials:         cfg.KeyRing,
			TLSServerName:       cfg.KubeTLSServerName,
			KubeClusters:        kubeCluster,
		}, cfg.KubeStoreAllCAs, writer); err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.BadParameter("unsupported identity format: %q, use one of %s", cfg.Format, KnownFileFormats)
	}
	return filesWritten, nil
}

func writeCassandraFormat(cfg WriteConfig, writer ConfigWriter) ([]string, error) {
	if cfg.Password == "" {
		pass, err := utils.CryptoRandomHex(16)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.Password = pass
	}
	// Cassandra expects a JKS keystore file with the private key and certificate
	// in it. The keystore file is password protected.
	keystoreBuf, err := prepareCassandraKeystore(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Cassandra expects a JKS truststore file with the CA certificate in it.
	// The truststore file is password protected.
	truststoreBuf, err := prepareCassandraTruststore(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certPath := cfg.OutputPath + ".keystore"
	casPath := cfg.OutputPath + ".truststore"
	err = writer.WriteFile(certPath, keystoreBuf.Bytes(), identityfile.FilePermissions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = writer.WriteFile(casPath, truststoreBuf.Bytes(), identityfile.FilePermissions)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []string{certPath, casPath}, nil
}

// writeOracleFormat creates an Oracle wallet files if orapki Oracle tool is available
// is user env otherwise creates a p12 key-pair file allowing to run orapki on the Oracle server
// and create the Oracle wallet manually.
func writeOracleFormat(cfg WriteConfig, writer ConfigWriter) ([]string, error) {
	certBlock, err := tlsca.ParseCertificatePEM(cfg.KeyRing.TLSCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyK, err := keys.ParsePrivateKey(cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// encode the private key and cert.
	// orapki import_pkcs12 refuses to add trusted certs unless they are an
	// issuer for an oracle wallet user_cert, and the server cert we create
	// is not signed by the DB Client CA, so don't pass trusted certs
	// (DB Client CA) here.
	pf, err := pkcs12.LegacyRC2.WithRand(rand.Reader).Encode(keyK.Signer, certBlock, nil, cfg.Password)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p12Path := cfg.OutputPath + ".p12"
	if err := writer.WriteFile(p12Path, pf, identityfile.FilePermissions); err != nil {
		return nil, trace.Wrap(err)
	}

	clientCAs := cfg.KeyRing.TLSCAs()
	var caPaths []string
	for i, caPEM := range clientCAs {
		var caPath string
		if len(clientCAs) > 1 {
			// orapki wallet add can only add one trusted cert at a time, so we
			// output up to two files - one for each CA key to trust during a
			// rotation.
			caPath = fmt.Sprintf("%s.ca-client-%d.crt", cfg.OutputPath, i)
		} else {
			caPath = cfg.OutputPath + ".ca-client.crt"
		}
		err = writer.WriteFile(caPath, caPEM, identityfile.FilePermissions)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		caPaths = append(caPaths, caPath)
	}

	// Is ORAPKI binary is available is user env run command ang generate autologin Oracle wallet.
	if isOrapkiAvailable() {
		// Is Orapki is available in the user env create the Oracle wallet directly.
		// otherwise Orapki tool needs to be executed on the server site to import keypair to
		// Oracle wallet.
		if err := createOracleWallet(caPaths, cfg.OutputPath, p12Path, cfg.Password); err != nil {
			return nil, trace.Wrap(err)
		}
		// If Oracle Wallet was created the raw p12 keypair and trusted cert are no longer needed.
		if err := os.Remove(p12Path); err != nil {
			return nil, trace.Wrap(err)
		}
		for _, caPath := range caPaths {
			if err := os.Remove(caPath); err != nil {
				return nil, trace.Wrap(err)
			}
		}
		// Return the path to the Oracle wallet.
		return []string{cfg.OutputPath}, nil
	}

	// Otherwise return destinations to p12 keypair and trusted CA allowing a user to run the convert flow on the
	// Oracle server instance in order to create Oracle wallet file.
	return append([]string{p12Path}, caPaths...), nil
}

const (
	orapkiBinary = "orapki"
)

func isOrapkiAvailable() bool {
	_, err := exec.LookPath(orapkiBinary)
	return err == nil
}

func createOracleWallet(caCertPaths []string, walletPath, p12Path, password string) error {
	errDetailsFormat := "\n\nOrapki command:\n%s \n\nCompleted with following error: \n%s"
	// Create Raw Oracle wallet with auto_login_only flag -  no password required.
	args := []string{
		"wallet", "create", "-wallet", walletPath,
		"-auto_login_only",
	}
	cmd := exec.Command(orapkiBinary, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return trace.Wrap(err, fmt.Sprintf(errDetailsFormat, cmd.String(), output))
	}

	// Import keypair into oracle wallet as a user cert.
	args = []string{
		"wallet", "import_pkcs12", "-wallet", walletPath,
		"-auto_login_only",
		"-pkcs12file", p12Path,
		"-pkcs12pwd", password,
	}
	cmd = exec.Command(orapkiBinary, args...)
	if output, err := exec.Command(orapkiBinary, args...).CombinedOutput(); err != nil {
		return trace.Wrap(err, fmt.Sprintf(errDetailsFormat, cmd.String(), output))
	}

	// Add import teleport CA(s) to the oracle wallet.
	for _, certPath := range caCertPaths {
		args = []string{
			"wallet", "add", "-wallet", walletPath,
			"-trusted_cert",
			"-auto_login_only",
			"-cert", certPath,
		}
		cmd = exec.Command(orapkiBinary, args...)
		if output, err := exec.Command(orapkiBinary, args...).CombinedOutput(); err != nil {
			return trace.Wrap(err, fmt.Sprintf(errDetailsFormat, cmd.String(), output))
		}
	}
	return nil
}

func prepareCassandraTruststore(cfg WriteConfig) (*bytes.Buffer, error) {
	var caCerts []byte
	for _, ca := range cfg.KeyRing.TrustedCerts {
		for _, cert := range ca.TLSCertificates {
			block, _ := pem.Decode(cert)
			caCerts = append(caCerts, block.Bytes...)
		}
	}

	ks := keystore.New()
	trustIn := keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate: keystore.Certificate{
			Type:    "x509",
			Content: caCerts,
		},
	}
	if err := ks.SetTrustedCertificateEntry("cassandra", trustIn); err != nil {
		return nil, trace.Wrap(err)
	}
	var buff bytes.Buffer
	if err := ks.Store(&buff, []byte(cfg.Password)); err != nil {
		return nil, trace.Wrap(err)
	}
	return &buff, nil
}

func prepareCassandraKeystore(cfg WriteConfig) (*bytes.Buffer, error) {
	certBlock, _ := pem.Decode(cfg.KeyRing.TLSCert)
	privBlock, _ := pem.Decode(cfg.KeyRing.TLSPrivateKey.PrivateKeyPEM())

	privKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privKeyPkcs8, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ks := keystore.New()
	pkeIn := keystore.PrivateKeyEntry{
		CreationTime: time.Now(),
		PrivateKey:   privKeyPkcs8,
		CertificateChain: []keystore.Certificate{
			{
				Type:    "x509",
				Content: certBlock.Bytes,
			},
		},
	}
	if err := ks.SetPrivateKeyEntry("cassandra", pkeIn, []byte(cfg.Password)); err != nil {
		return nil, trace.Wrap(err)
	}
	var buff bytes.Buffer
	if err := ks.Store(&buff, []byte(cfg.Password)); err != nil {
		return nil, trace.Wrap(err)
	}
	return &buff, nil
}

func checkOverwrite(ctx context.Context, writer ConfigWriter, force bool, paths ...string) error {
	var existingFiles []string
	// Check if the destination file exists.
	for _, path := range paths {
		_, err := writer.Stat(path)
		if os.IsNotExist(err) {
			// File doesn't exist, proceed.
			continue
		}
		if err != nil {
			// Something else went wrong, fail.
			return trace.ConvertSystemError(err)
		}
		existingFiles = append(existingFiles, path)
	}
	if len(existingFiles) == 0 || force {
		// Files don't exist or we're asked not to prompt, proceed.
		return nil
	}

	// Some files exist, prompt user whether to overwrite.
	overwrite, err := prompt.Confirmation(ctx, os.Stderr, prompt.Stdin(), fmt.Sprintf("Destination file(s) %s exist. Overwrite?", strings.Join(existingFiles, ", ")))
	if err != nil {
		return trace.Wrap(err)
	}
	if !overwrite {
		return trace.AlreadyExists("not overwriting destination files %s", strings.Join(existingFiles, ", "))
	}
	return nil
}

// KeyRingFromIdentityFile loads client key ring from an identity file.
func KeyRingFromIdentityFile(identityPath, proxyHost, clusterName string) (*client.KeyRing, error) {
	if proxyHost == "" {
		return nil, trace.BadParameter("proxyHost must be provided to parse identity file")
	}
	ident, err := identityfile.ReadFile(identityPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse identity file")
	}

	priv, err := keys.ParsePrivateKey(ident.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Identity file uses same private key for SSH and TLS.
	keyRing := client.NewKeyRing(priv, priv)
	keyRing.Cert = ident.Certs.SSH
	keyRing.TLSCert = ident.Certs.TLS
	keyRing.KeyRingIndex = client.KeyRingIndex{
		ProxyHost:   proxyHost,
		ClusterName: clusterName,
	}

	// validate TLS Cert (if present):
	if len(ident.Certs.TLS) > 0 {
		certDERBlock, _ := pem.Decode(ident.Certs.TLS)
		cert, err := x509.ParseCertificate(certDERBlock.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if keyRing.ClusterName == "" {
			keyRing.ClusterName = cert.Issuer.CommonName
		}

		parsedIdent, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keyRing.Username = parsedIdent.Username

		// If this identity file has any database certs, copy it into the DBTLSCerts map.
		if parsedIdent.RouteToDatabase.ServiceName != "" {
			keyRing.DBTLSCredentials[parsedIdent.RouteToDatabase.ServiceName] = client.TLSCredential{
				// Identity files only have room for one private key, it must match the db cert.
				PrivateKey: priv,
				Cert:       ident.Certs.TLS,
			}
		}

		// Similarly, if this identity has any app certs, copy them in.
		if parsedIdent.RouteToApp.Name != "" {
			keyRing.AppTLSCredentials[parsedIdent.RouteToApp.Name] = client.TLSCredential{
				// Identity files only have room for one private key and TLS
				// cert, it must match the app cert.
				PrivateKey: priv,
				Cert:       ident.Certs.TLS,
			}
		}

		// If this identity file has any kubernetes certs, copy it into the
		// KubeTLSCerts map.
		if parsedIdent.KubernetesCluster != "" {
			keyRing.KubeTLSCredentials[parsedIdent.KubernetesCluster] = client.TLSCredential{
				// Identity files only have room for one private key, it must
				// match the kube cert.
				PrivateKey: priv,
				Cert:       ident.Certs.TLS,
			}
		}
	} else {
		keyRing.Username, err = keyRing.CertUsername()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	knownHosts, err := sshutils.UnmarshalKnownHosts(ident.CACerts.SSH)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use all Trusted certs found in the identity file.
	keyRing.TrustedCerts, err = client.TrustedCertsFromCACerts(ident.CACerts.TLS, knownHosts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return keyRing, nil
}

// NewClientStoreFromIdentityFile initializes a new in-memory client store
// and loads data from the given identity file into it. A temporary profile
// is also added to its profile store with the limited profile data available
// in the identity file.
//
// Use [proxyAddr] to specify the host:port-like address of the proxy.
// This is necessary because identity files do not store the proxy address.
// Additionally, the [clusterName] argument can ve used to target a leaf cluster
// rather than the default root cluster.
func NewClientStoreFromIdentityFile(identityFile, proxyAddr, clusterName string, hwKeyService keys.HardwareKeyService) (*client.Store, error) {
	clientStore := client.NewMemClientStore(hwKeyService)
	if err := LoadIdentityFileIntoClientStore(clientStore, identityFile, proxyAddr, clusterName); err != nil {
		return nil, trace.Wrap(err)
	}

	return clientStore, nil
}

// LoadIdentityFileIntoClientStore loads the identityFile from the given path
// into the given client store, assimilating it with other keys in the store.
func LoadIdentityFileIntoClientStore(store *client.Store, identityFile, proxyAddr, clusterName string) error {
	if proxyAddr == "" {
		return trace.BadParameter("missing a Proxy address when loading an Identity File.")
	}
	proxyHost, err := utils.Host(proxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	keyRing, err := KeyRingFromIdentityFile(identityFile, proxyHost, clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// This key may already exist in a tsh profile, delete it before overwriting
	// it with the identity key to avoid leaving app/db/kube certs from the old key.
	if err := store.DeleteKeyRing(keyRing.KeyRingIndex); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if err := store.AddKeyRing(keyRing); err != nil {
		return trace.Wrap(err)
	}

	// If the client store is not initialized (this is the first key added),
	// then the profile will be missing. Add a temporary profile with necessary
	// information. This profile should be filled in with information from the
	// proxy by the caller.
	if _, err := store.GetProfile(proxyAddr); trace.IsNotFound(err) {
		profile := &profile.Profile{
			MissingClusterDetails: true,
			WebProxyAddr:          proxyAddr,
			SiteName:              keyRing.ClusterName,
			Username:              keyRing.Username,
			PrivateKeyPolicy:      keyRing.TLSPrivateKey.GetPrivateKeyPolicy(),
		}
		if err := store.SaveProfile(profile, true); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
