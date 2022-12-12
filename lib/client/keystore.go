/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bufio"
	"context"
	"encoding/pem"
	"fmt"
	osfs "io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// profileDirPerms is the default permissions applied to the profile
	// directory (usually ~/.tsh)
	profileDirPerms os.FileMode = 0700

	// keyFilePerms is the default permissions applied to key files (.cert, .key, pub)
	// under ~/.tsh
	keyFilePerms os.FileMode = 0600

	// tshConfigFileName is the name of the directory containing the
	// tsh config file.
	tshConfigFileName = "config"

	// tshAzureDirName is the name of the directory containing the
	// az cli app-specific profiles.
	tshAzureDirName = "azure"
)

// ClientStore is a storage interface for client data. ClientStore is made up three types
// of data stores.
//
// A ClientStore can be made up of partial data stores with different backends. For example,
// when using `tsh --add-keys-to-agent=only`, ClientStore will be made up of an in-memory
// key store and an FS (~/.tsh) profile and trusted certs store.
type ClientStore interface {
	ClientKeyStore
	ClientTrustedCAStore
	ClientProfileStore
}

// ClientKeyStore is a storage interface for client session keys and certificates.
type ClientKeyStore interface {
	// AddKey adds the given key to the store.
	AddKey(key *Key) error

	// GetKey returns the user's key including the specified certs.
	GetKey(idx KeyIndex, opts ...CertOption) (*Key, error)

	// DeleteKey deletes the user's key with all its certs.
	DeleteKey(idx KeyIndex) error

	// DeleteUserCerts deletes only the specified certs of the user's key,
	// keeping the private key intact.
	DeleteUserCerts(idx KeyIndex, opts ...CertOption) error

	// DeleteKeys removes all session keys.
	DeleteKeys() error

	// GetSSHCertificates gets all certificates signed for the given user and proxy,
	// including certificates for trusted clusters.
	GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error)
}

// ClientTrustedCAStore is a storage interface for trusted CA certificates and public keys.
type ClientTrustedCAStore interface {
	// AddTrustedCerts adds the given trusted CA TLS certificates and SSH host keys to the store.
	AddTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error

	// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
	GetTrustedCerts(proxyHost string) ([]auth.TrustedCerts, error)

	// GetTrustedCertsPEM gets trusted TLS certificates of certificate authorities.
	// Each returned byte slice contains an individual PEM block.
	GetTrustedCertsPEM(proxyHost string) ([][]byte, error)

	// GetKnownHostKeys returns all known public host keys for the given cluster names.
	GetKnownHostKeys(clusterNames ...string) ([]ssh.PublicKey, error)
}

// HybridClientStore implements ClientStore using three separate partial stores.
type HybridClientStore struct {
	ClientKeyStore
	ClientTrustedCAStore
	ClientProfileStore
}

// NewClientStore creates a new ClientStore using the provided stores.
func NewClientStore(ks ClientKeyStore, ns ClientTrustedCAStore, ps ClientProfileStore) ClientStore {
	return &HybridClientStore{
		ClientKeyStore:       ks,
		ClientTrustedCAStore: ns,
		ClientProfileStore:   ps,
	}
}

// FSClientStore is an on-disk implementation of the ClientStore interface.
//
// The FS store uses the file layout outlined in `api/utils/keypaths.go`.
type FSClientStore struct {
	// log holds the structured logger.
	log logrus.FieldLogger

	// KeyDir is the directory where all keys are stored.
	KeyDir string
}

// NewFSClientStore intitializes a new FSClientStore.
//
// If dirPath is empty, sets it to ~/.tsh.
func NewFSClientStore(dirPath string) (s *FSClientStore, err error) {
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FSClientStore{
		log:    logrus.WithField(trace.Component, teleport.ComponentKeyStore),
		KeyDir: dirPath,
	}, nil
}

// initKeysDir initializes the keystore root directory, usually `~/.tsh`.
func initKeysDir(dirPath string) (string, error) {
	dirPath = profile.FullProfilePath(dirPath)
	if err := os.MkdirAll(dirPath, os.ModeDir|profileDirPerms); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return dirPath, nil
}

// proxyKeyDir returns the keystore's keys directory for the given proxy.
func (fs *FSClientStore) proxyKeyDir(proxy string) string {
	return keypaths.ProxyKeyDir(fs.KeyDir, proxy)
}

// casDir returns path to trusted clusters certificates directory.
func (fs *FSClientStore) casDir(proxy string) string {
	return keypaths.CAsDir(fs.KeyDir, proxy)
}

// clusterCAPath returns path to cluster certificate.
func (fs *FSClientStore) clusterCAPath(proxy, clusterName string) string {
	return keypaths.TLSCAsPathCluster(fs.KeyDir, proxy, clusterName)
}

// knownHostsPath returns the keystore's known hosts file path.
func (fs *FSClientStore) knownHostsPath() string {
	return keypaths.KnownHostsPath(fs.KeyDir)
}

// userKeyPath returns the private key path for the given KeyIndex.
func (fs *FSClientStore) userKeyPath(idx KeyIndex) string {
	return keypaths.UserKeyPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// tlsCertPath returns the TLS certificate path given KeyIndex.
func (fs *FSClientStore) tlsCertPath(idx KeyIndex) string {
	return keypaths.TLSCertPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// tlsCAsPath returns the TLS CA certificates path for the given KeyIndex.
func (fs *FSClientStore) tlsCAsPath(proxy string) string {
	return keypaths.TLSCAsPath(fs.KeyDir, proxy)
}

// sshDir returns the SSH certificate path for the given KeyIndex.
func (fs *FSClientStore) sshDir(proxy, user string) string {
	return keypaths.SSHDir(fs.KeyDir, proxy, user)
}

// sshCertPath returns the SSH certificate path for the given KeyIndex.
func (fs *FSClientStore) sshCertPath(idx KeyIndex) string {
	return keypaths.SSHCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
}

// ppkFilePath returns the PPK (PuTTY-formatted) keypair path for the given KeyIndex.
func (fs *FSClientStore) ppkFilePath(idx KeyIndex) string {
	return keypaths.PPKFilePath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// publicKeyPath returns the public key path for the given KeyIndex.
func (fs *FSClientStore) publicKeyPath(idx KeyIndex) string {
	return keypaths.PublicKeyPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// appCertPath returns the TLS certificate path for the given KeyIndex and app name.
func (fs *FSClientStore) appCertPath(idx KeyIndex, appname string) string {
	return keypaths.AppCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, appname)
}

// databaseCertPath returns the TLS certificate path for the given KeyIndex and database name.
func (fs *FSClientStore) databaseCertPath(idx KeyIndex, dbname string) string {
	return keypaths.DatabaseCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, dbname)
}

// kubeCertPath returns the TLS certificate path for the given KeyIndex and kube cluster name.
func (fs *FSClientStore) kubeCertPath(idx KeyIndex, kubename string) string {
	return keypaths.KubeCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, kubename)
}

// AddKey adds the given key to the store.
func (fs *FSClientStore) AddKey(key *Key) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}

	if err := fs.writeBytes(key.PrivateKeyPEM(), fs.userKeyPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}

	if err := fs.writeBytes(key.MarshalSSHPublicKey(), fs.publicKeyPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}

	// Store TLS cert
	if err := fs.writeBytes(key.TLSCert, fs.tlsCertPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}
	if runtime.GOOS == constants.WindowsOS {
		ppkFile, err := key.PPKFile()
		if err == nil {
			if err := fs.writeBytes(ppkFile, fs.ppkFilePath(key.KeyIndex)); err != nil {
				return trace.Wrap(err)
			}
		} else if !trace.IsBadParameter(err) {
			return trace.Wrap(err)
		}
		// PPKFile can only be generated from an RSA private key.
		fs.log.WithError(err).Debugf("Failed to convert private key to PPK-formatted keypair.")
	}

	// Store per-cluster key data.
	if len(key.Cert) > 0 {
		if err := fs.writeBytes(key.Cert, fs.sshCertPath(key.KeyIndex)); err != nil {
			return trace.Wrap(err)
		}
	}

	// TODO(awly): unit test this.
	for kubeCluster, cert := range key.KubeTLSCerts {
		// Prevent directory traversal via a crafted kubernetes cluster name.
		//
		// This will confuse cluster cert loading (GetKey will return
		// kubernetes cluster names different from the ones stored here), but I
		// don't expect any well-meaning user to create bad names.
		kubeCluster = filepath.Clean(kubeCluster)

		path := fs.kubeCertPath(key.KeyIndex, kubeCluster)
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}
	for db, cert := range key.DBTLSCerts {
		path := fs.databaseCertPath(key.KeyIndex, filepath.Clean(db))
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}
	for app, cert := range key.AppTLSCerts {
		path := fs.appCertPath(key.KeyIndex, filepath.Clean(app))
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (fs *FSClientStore) writeBytes(bytes []byte, fp string) error {
	if err := os.MkdirAll(filepath.Dir(fp), os.ModeDir|profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}
	err := os.WriteFile(fp, bytes, keyFilePerms)
	if err != nil {
		fs.log.Error(err)
	}
	return trace.ConvertSystemError(err)
}

// DeleteKey deletes the user's key with all its certs.
func (fs *FSClientStore) DeleteKey(idx KeyIndex) error {
	files := []string{
		fs.userKeyPath(idx),
		fs.publicKeyPath(idx),
		fs.tlsCertPath(idx),
	}
	for _, fn := range files {
		if err := os.Remove(fn); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	// we also need to delete the extra PuTTY-formatted .ppk file when running on Windows,
	// but it may not exist when upgrading from v9 -> v10 and logging into an existing cluster.
	// as such, deletion should be best-effort and not generate an error if it fails.
	if runtime.GOOS == constants.WindowsOS {
		os.Remove(fs.ppkFilePath(idx))
	}

	// Clear ClusterName to delete the user certs stored for all clusters.
	idx.ClusterName = ""
	return fs.DeleteUserCerts(idx, WithAllCerts...)
}

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
// Empty clusterName indicates to delete the certs for all clusters.
//
// Useful when needing to log out of a specific service, like a particular
// database proxy.
func (fs *FSClientStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	for _, o := range opts {
		certPath := o.certPath(fs.KeyDir, idx)
		if err := os.RemoveAll(certPath); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// DeleteKeys removes all session keys.
func (fs *FSClientStore) DeleteKeys() error {
	files, err := os.ReadDir(fs.KeyDir)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	for _, file := range files {
		// Don't delete 'config' and 'azure' directories.
		// TODO: this is hackish and really shouldn't be needed, but fs.KeyDir is `~/.tsh` while it probably should be `~/.tsh/keys` instead.
		if file.IsDir() && file.Name() == tshConfigFileName {
			continue
		}
		if file.IsDir() && file.Name() == tshAzureDirName {
			continue
		}
		if file.IsDir() {
			err := os.RemoveAll(filepath.Join(fs.KeyDir, file.Name()))
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			continue
		}
		err := os.Remove(filepath.Join(fs.KeyDir, file.Name()))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// GetKey returns the user's key including the specified certs.
// If the key is not found, returns trace.NotFound error.
func (fs *FSClientStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	if len(opts) > 0 {
		if err := idx.Check(); err != nil {
			return nil, trace.Wrap(err, "GetKey with CertOptions requires a fully specified KeyIndex")
		}
	}

	if _, err := os.ReadDir(fs.KeyDir); err != nil && trace.IsNotFound(err) {
		return nil, trace.Wrap(err, "no session keys for %+v", idx)
	}

	tlsCertFile := fs.tlsCertPath(idx)
	tlsCert, err := os.ReadFile(tlsCertFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}

	priv, err := keys.LoadKeyPair(fs.userKeyPath(idx), fs.publicKeyPath(idx))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}

	key := NewKey(priv)
	key.KeyIndex = idx
	key.TLSCert = tlsCert

	tlsCertExpiration, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fs.log.Debugf("Returning Teleport TLS certificate %q valid until %q.", tlsCertFile, tlsCertExpiration)

	for _, o := range opts {
		if err := fs.updateKeyWithCerts(o, key); err != nil && !trace.IsNotFound(err) {
			fs.log.Error(err)
			return nil, trace.Wrap(err)
		}
	}

	// Note, we may be returning expired certificates here, that is okay. If a
	// certificate is expired, it's the responsibility of the TeleportClient to
	// perform cleanup of the certificates and the profile.

	return key, nil
}

// GetSSHCertificates gets all certificates signed for the given user and proxy.
func (fs *FSClientStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
	certDir := fs.sshDir(proxyHost, username)
	certFiles, err := os.ReadDir(certDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshCerts := make([]*ssh.Certificate, len(certFiles))
	for i, certFile := range certFiles {
		data, err := os.ReadFile(filepath.Join(certDir, certFile.Name()))
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}

		sshCerts[i], err = apisshutils.ParseCertificate(data)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return sshCerts, nil
}

// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
func (fs *FSClientStore) GetTrustedCerts(proxyHost string) ([]auth.TrustedCerts, error) {
	tlsCA, err := fs.GetTrustedCertsPEM(proxyHost)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	knownHosts, err := fs.getKnownHostsFile()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return trustedCertsFromCACerts(tlsCA, [][]byte{knownHosts})
}

func (fs *FSClientStore) getAuthorizedKeys(proxyHost, clusterName string) ([][]byte, error) {
	sshCAHostKeys, err := fs.GetKnownHostKeys(proxyHost, clusterName)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}
	hostCerts := make([][]byte, len(sshCAHostKeys))
	for i, hostKey := range sshCAHostKeys {
		hostCerts[i] = ssh.MarshalAuthorizedKey(hostKey)
	}
	return hostCerts, nil
}

func (fs *FSClientStore) updateKeyWithCerts(o CertOption, key *Key) error {
	certPath := o.certPath(fs.KeyDir, key.KeyIndex)
	info, err := os.Stat(certPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	fs.log.Debugf("Reading certificates from path %q.", certPath)

	if info.IsDir() {
		certDataMap := map[string][]byte{}
		certFiles, err := os.ReadDir(certPath)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		for _, certFile := range certFiles {
			name := keypaths.TrimCertPathSuffix(certFile.Name())
			if isCert := name != certFile.Name(); isCert {
				data, err := os.ReadFile(filepath.Join(certPath, certFile.Name()))
				if err != nil {
					return trace.ConvertSystemError(err)
				}
				certDataMap[name] = data
			}
		}
		return o.updateKeyWithMap(key, certDataMap)
	}

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return o.updateKeyWithBytes(key, certBytes)
}

// GetKnownHostKeys returns all known public keys from `known_hosts`.
func (fs *FSClientStore) GetKnownHostKeys(clusterNames ...string) (keys []ssh.PublicKey, retErr error) {
	knownHosts, err := fs.getKnownHostsFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Return all known host keys with one of the given cluster names or proxyHost as a hostname.
	return apisshutils.ParseKnownHosts([][]byte{knownHosts}, clusterNames...)
}

func (fs *FSClientStore) getKnownHostsFile() (knownHosts []byte, retErr error) {
	unlock, err := utils.FSTryReadLockTimeout(context.Background(), fs.knownHostsPath(), 5*time.Second)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "could not acquire lock for the `known_hosts` file")
	}
	defer utils.StoreErrorOf(unlock, &retErr)

	knownHosts, err = os.ReadFile(fs.knownHostsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	return knownHosts, nil
}

// AddTrustedCerts saves trusted TLS certificates of certificate authorities.
func (fs *FSClientStore) AddTrustedCerts(proxyHost string, cas []auth.TrustedCerts) (retErr error) {
	if proxyHost == "" {
		return trace.BadParameter("proxyHost must be provided to add trusted certs")
	}

	if err := os.MkdirAll(fs.proxyKeyDir(proxyHost), os.ModeDir|profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}

	for _, ca := range cas {
		if ca.ClusterName == "" {
			return trace.BadParameter("ca entry cannot have an empty cluster name")
		}
	}

	// Save trusted clusters certs in CAS directory.
	if err := fs.saveTrustedCertsInCASDir(proxyHost, cas); err != nil {
		return trace.Wrap(err)
	}

	// For backward compatibility save trusted in legacy certs.pem file.
	if err := fs.saveTrustedCertsInLegacyCAFile(proxyHost, cas); err != nil {
		return trace.Wrap(err)
	}

	if err := fs.saveKnownHosts(proxyHost, cas); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (fs *FSClientStore) saveTrustedCertsInCASDir(proxyHost string, cas []auth.TrustedCerts) error {
	casDirPath := filepath.Join(fs.casDir(proxyHost))
	if err := os.MkdirAll(casDirPath, os.ModeDir|profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}

	for _, ca := range cas {
		if len(ca.TLSCertificates) == 0 {
			continue
		}
		if !isSafeClusterName(ca.ClusterName) {
			fs.log.Warnf("Skipped unsafe cluster name: %q", ca.ClusterName)
			continue
		}
		// Create CA files in cas dir for each cluster.
		caFile, err := os.OpenFile(fs.clusterCAPath(proxyHost, ca.ClusterName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
		if err != nil {
			return trace.ConvertSystemError(err)
		}

		if err := writeClusterCertificates(caFile, ca.TLSCertificates); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (fs *FSClientStore) saveTrustedCertsInLegacyCAFile(proxyHost string, cas []auth.TrustedCerts) (retErr error) {
	certsFile := fs.tlsCAsPath(proxyHost)
	fp, err := os.OpenFile(certsFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer utils.StoreErrorOf(fp.Close, &retErr)
	for _, ca := range cas {
		for _, cert := range ca.TLSCertificates {
			if _, err := fp.Write(cert); err != nil {
				return trace.ConvertSystemError(err)
			}
			if _, err := fmt.Fprintln(fp); err != nil {
				return trace.ConvertSystemError(err)
			}
		}
	}
	return fp.Sync()
}

// isSafeClusterName check if cluster name is safe and doesn't contain miscellaneous characters.
func isSafeClusterName(name string) bool {
	return !strings.Contains(name, "..")
}

func writeClusterCertificates(f *os.File, tlsCertificates [][]byte) error {
	defer f.Close()
	for _, cert := range tlsCertificates {
		if _, err := f.Write(cert); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	if err := f.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// saveKnownHosts adds new entries to `known_hosts` file for the provided CAs.
func (fs *FSClientStore) saveKnownHosts(proxyHost string, cas []auth.TrustedCerts) (retErr error) {
	// We're trying to serialize our writes to the 'known_hosts' file to avoid corruption, since there
	// are cases when multiple tsh instances will try to write to it.
	unlock, err := utils.FSTryWriteLockTimeout(context.Background(), fs.knownHostsPath(), 5*time.Second)
	if err != nil {
		return trace.WrapWithMessage(err, "could not acquire lock for the `known_hosts` file")
	}
	defer utils.StoreErrorOf(unlock, &retErr)

	fp, err := os.OpenFile(fs.knownHostsPath(), os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer utils.StoreErrorOf(fp.Close, &retErr)
	// read all existing entries into a map (this removes any pre-existing dupes)
	entries := make(map[string]int)
	output := make([]string, 0)
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text()
		if _, exists := entries[line]; !exists {
			output = append(output, line)
			entries[line] = 1
		}
	}
	// check if the scanner ran into an error
	if err := scanner.Err(); err != nil {
		return trace.Wrap(err)
	}

	// add every host key to the list of entries
	for _, ca := range cas {
		for _, hostKey := range ca.HostCertificates {
			fs.log.Debugf("Adding known host %s with proxy %s", ca.ClusterName, proxyHost)

			// Write keys in an OpenSSH-compatible format. A previous format was not
			// quite OpenSSH-compatible, so we may write a duplicate entry here. Any
			// duplicates will be pruned below.
			// We include both the proxy server and original hostname as well as the
			// root domain wildcard. OpenSSH clients match against both the proxy
			// host and nodes (via the wildcard). Teleport itself occasionally uses
			// the root cluster name.
			line := sshutils.MarshalAuthorizedHostsFormat(ca.ClusterName, proxyHost, hostKey)

			if _, exists := entries[line]; !exists {
				output = append(output, line)
			}
		}
	}

	// Prune any duplicate host entries for migrated hosts. Note that only
	// duplicates matching the current hostname/proxyHost will be pruned; others
	// will be cleaned up at subsequent logins.
	output = pruneOldHostKeys(output)
	// re-create the file:
	_, err = fp.Seek(0, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = fp.Truncate(0); err != nil {
		return trace.Wrap(err)
	}
	for _, line := range output {
		fmt.Fprintf(fp, "%s\n", line)
	}
	return fp.Sync()
}

// GetTrustedCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (fs *FSClientStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	dir := fs.casDir(proxyHost)

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, trace.NotFound("please relogin, tsh user profile doesn't contain CAS directory: %s", dir)
		}
		return nil, trace.ConvertSystemError(err)
	}

	var blocks [][]byte
	err := filepath.Walk(dir, func(path string, info osfs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		for len(data) > 0 {
			if err != nil {
				return trace.Wrap(err)
			}
			block, rest := pem.Decode(data)
			if block == nil {
				break
			}
			if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
				fs.log.Debugf("Skipping PEM block type=%v headers=%v.", block.Type, block.Headers)
				data = rest
				continue
			}
			// rest contains the remainder of data after reading a block.
			// Therefore, the block length is len(data) - len(rest).
			// Use that length to slice the block from the start of data.
			blocks = append(blocks, data[:len(data)-len(rest)])
			data = rest
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return blocks, nil
}

func (fs *FSClientStore) CurrentProfile() (string, error) {
	profileName, err := profile.GetCurrentProfileName(fs.KeyDir)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return profileName, nil
}

func (fs *FSClientStore) ListProfiles() ([]string, error) {
	profileNames, err := profile.ListProfileNames(fs.KeyDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profileNames, nil
}

func (fs *FSClientStore) GetProfile(profileName string) (*profile.Profile, error) {
	profile, err := profile.FromDir(fs.KeyDir, profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profile, nil
}

// SaveProfile saves this profile in the keystore.
// If makeCurrent is true, it makes this profile current.
func (fs *FSClientStore) SaveProfile(profile *profile.Profile, makeCurrent bool) error {
	err := profile.SaveToDir(fs.KeyDir, makeCurrent)
	return trace.Wrap(err)
}

// CertOption is an additional step to run when loading/deleting user certificates.
type CertOption interface {
	// certPath returns a path to the cert (or to a dir holding the certs)
	// within the given key dir. For use with FSLocalKeyStore.
	certPath(keyDir string, idx KeyIndex) string
	// updateKeyWithBytes adds the cert bytes to the key and performs related checks.
	updateKeyWithBytes(key *Key, certBytes []byte) error
	// updateKeyWithMap adds the cert data map to the key and performs related checks.
	updateKeyWithMap(key *Key, certMap map[string][]byte) error
	// deleteFromKey deletes the cert data from the key.
	deleteFromKey(key *Key)
}

// WithAllCerts lists all known CertOptions.
var WithAllCerts = []CertOption{WithSSHCerts{}, WithKubeCerts{}, WithDBCerts{}, WithAppCerts{}}

// WithSSHCerts is a CertOption for handling SSH certificates.
type WithSSHCerts struct{}

func (o WithSSHCerts) certPath(keyDir string, idx KeyIndex) string {
	if idx.ClusterName == "" {
		return keypaths.SSHDir(keyDir, idx.ProxyHost, idx.Username)
	}
	return keypaths.SSHCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
}

func (o WithSSHCerts) updateKeyWithBytes(key *Key, certBytes []byte) error {
	key.Cert = certBytes

	// Validate the SSH certificate.
	if err := key.CheckCert(); err != nil {
		if !utils.IsCertExpiredError(err) {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (o WithSSHCerts) updateKeyWithMap(key *Key, certMap map[string][]byte) error {
	return trace.NotImplemented("WithSSHCerts does not implement updateKeyWithMap")
}

func (o WithSSHCerts) deleteFromKey(key *Key) {
	key.Cert = nil
}

// WithKubeCerts is a CertOption for handling kubernetes certificates.
type WithKubeCerts struct{}

func (o WithKubeCerts) certPath(keyDir string, idx KeyIndex) string {
	if idx.ClusterName == "" {
		return keypaths.KubeDir(keyDir, idx.ProxyHost, idx.Username)
	}
	return keypaths.KubeCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
}

func (o WithKubeCerts) updateKeyWithBytes(key *Key, certBytes []byte) error {
	return trace.NotImplemented("WithKubeCerts does not implement updateKeyWithBytes")
}

func (o WithKubeCerts) updateKeyWithMap(key *Key, certMap map[string][]byte) error {
	key.KubeTLSCerts = certMap
	return nil
}

func (o WithKubeCerts) deleteFromKey(key *Key) {
	key.KubeTLSCerts = nil
}

// WithDBCerts is a CertOption for handling database access certificates.
type WithDBCerts struct {
	dbName string
}

func (o WithDBCerts) certPath(keyDir string, idx KeyIndex) string {
	if idx.ClusterName == "" {
		return keypaths.DatabaseDir(keyDir, idx.ProxyHost, idx.Username)
	}
	if o.dbName == "" {
		return keypaths.DatabaseCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	}
	return keypaths.DatabaseCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName, o.dbName)
}

func (o WithDBCerts) updateKeyWithBytes(key *Key, certBytes []byte) error {
	return trace.NotImplemented("WithDBCerts does not implement updateKeyWithBytes")
}

func (o WithDBCerts) updateKeyWithMap(key *Key, certMap map[string][]byte) error {
	key.DBTLSCerts = certMap
	return nil
}

func (o WithDBCerts) deleteFromKey(key *Key) {
	key.DBTLSCerts = nil
}

// WithAppCerts is a CertOption for handling application access certificates.
type WithAppCerts struct {
	appName string
}

func (o WithAppCerts) certPath(keyDir string, idx KeyIndex) string {
	if idx.ClusterName == "" {
		return keypaths.AppDir(keyDir, idx.ProxyHost, idx.Username)
	}
	if o.appName == "" {
		return keypaths.AppCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	}
	return keypaths.AppCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName, o.appName)
}

func (o WithAppCerts) updateKeyWithBytes(key *Key, certBytes []byte) error {
	return trace.NotImplemented("WithAppCerts does not implement updateKeyWithBytes")
}

func (o WithAppCerts) updateKeyWithMap(key *Key, certMap map[string][]byte) error {
	key.AppTLSCerts = certMap
	return nil
}

func (o WithAppCerts) deleteFromKey(key *Key) {
	key.AppTLSCerts = nil
}

// noClientStore is a ClientStore representing the absence of a ClientStore.
// All methods return errors. This exists to avoid nil checking everywhere in
// LocalKeyAgent and prevent nil pointer panics.
type noClientStore struct{}

var errNoClientStore = trace.NotFound("there is no client store")

func (noClientStore) CurrentProfile() (string, error) {
	return "", errNoClientStore
}
func (noClientStore) ListProfiles() ([]string, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetProfile(profileName string) (*profile.Profile, error) {
	return nil, errNoClientStore
}
func (noClientStore) SaveProfile(*profile.Profile, bool) error {
	return errNoClientStore
}
func (noClientStore) AddKey(key *Key) error {
	return errNoClientStore
}
func (noClientStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	return nil, errNoClientStore
}
func (noClientStore) DeleteKey(idx KeyIndex) error {
	return errNoClientStore
}
func (noClientStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	return errNoClientStore
}
func (noClientStore) DeleteKeys() error {
	return errNoClientStore
}
func (noClientStore) AddTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error {
	return errNoClientStore
}
func (noClientStore) GetTrustedCerts(proxyHost string) ([]auth.TrustedCerts, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetKnownHostKeys(clusterNames ...string) ([]ssh.PublicKey, error) {
	return nil, errNoClientStore
}
func (noClientStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
	return nil, errNoClientStore
}

// MemClientStore is an in-memory ClientStore implementation.
type MemClientStore struct {
	log *logrus.Entry
	// keys is a three-dimensional map indexed by [proxyHost][username][clusterName]
	keys map[string]map[string]map[string]*Key
	// memLocalCAStoreMap is a two-dimensinoal map indexed by [proxyHost][clusterName]
	trustedCAs map[string]map[string]*auth.TrustedCerts
	// currentProfile is the currently selected profile.
	currentProfile string
	// profiles is a map of proxyHosts to profiles.
	profiles map[string]*profile.Profile
}

// NewMemClientStore initializes a MemClientStore.
func NewMemClientStore() *MemClientStore {
	return &MemClientStore{
		log:        logrus.WithField(trace.Component, teleport.ComponentKeyStore),
		keys:       make(map[string]map[string]map[string]*Key),
		trustedCAs: make(map[string]map[string]*auth.TrustedCerts),
		profiles:   make(map[string]*profile.Profile),
	}
}

// AddKey writes a key to the underlying key store.
func (ms *MemClientStore) AddKey(key *Key) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, ok := ms.keys[key.ProxyHost]
	if !ok {
		ms.keys[key.ProxyHost] = map[string]map[string]*Key{}
	}
	_, ok = ms.keys[key.ProxyHost][key.Username]
	if !ok {
		ms.keys[key.ProxyHost][key.Username] = map[string]*Key{}
	}
	ms.keys[key.ProxyHost][key.Username][key.ClusterName] = key.Clone()
	// TrustedCA is stored separately in the Memory store so we wipe out
	// the keys' trusted CA to prevent inconsistencies.
	key.TrustedCerts = nil

	return nil
}

// GetKey returns the user's key including the specified certs.
func (ms *MemClientStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	// If clusterName is not specified then the cluster-dependent fields
	// are not considered relevant and we may simply return any key
	// associated with any cluster name whatsoever.
	if idx.ClusterName == "" {
		for clusterName := range ms.keys[idx.ProxyHost][idx.Username] {
			idx.ClusterName = clusterName
			break
		}
	}

	key, ok := ms.keys[idx.ProxyHost][idx.Username][idx.ClusterName]
	if !ok {
		return nil, trace.NotFound("key for %+v not found", idx)
	}

	// It is not necessary to handle opts because all the optional certs are
	// already part of the Key struct as stored in memory.

	tlsCertExpiration, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ms.log.Debugf("Returning Teleport TLS certificate from memory, valid until %q.", tlsCertExpiration)

	// Validate the SSH certificate.
	if err := key.CheckCert(); err != nil {
		if !utils.IsCertExpiredError(err) {
			return nil, trace.Wrap(err)
		}
	}

	return key.Clone(), nil
}

// DeleteKey deletes the user's key with all its certs.
func (ms *MemClientStore) DeleteKey(idx KeyIndex) error {
	delete(ms.keys[idx.ProxyHost], idx.Username)
	return nil
}

// DeleteKeys removes all session keys.
func (ms *MemClientStore) DeleteKeys() error {
	ms.keys = make(map[string]map[string]map[string]*Key)
	return nil
}

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
// Empty clusterName indicates to delete the certs for all clusters.
//
// Useful when needing to log out of a specific service, like a particular
// database proxy.
func (ms *MemClientStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	var keys []*Key
	if idx.ClusterName != "" {
		key, ok := ms.keys[idx.ProxyHost][idx.Username][idx.ClusterName]
		if !ok {
			return nil
		}
		keys = []*Key{key}
	} else {
		keys = make([]*Key, 0, len(ms.keys[idx.ProxyHost][idx.Username]))
		for _, key := range ms.keys[idx.ProxyHost][idx.Username] {
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		for _, o := range opts {
			o.deleteFromKey(key)
		}
	}
	return nil
}

// GetSSHCertificates gets all certificates signed for the given user and proxy.
func (ms *MemClientStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
	var sshCerts []*ssh.Certificate
	for _, key := range ms.keys[proxyHost][username] {
		sshCert, err := key.SSHCert()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sshCerts = append(sshCerts, sshCert)
	}

	return sshCerts, nil
}

// AddTrustedCerts saves trusted TLS certificates of certificate authorities.
func (ms *MemClientStore) AddTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error {
	if proxyHost == "" {
		return trace.BadParameter("proxyHost must be provided to add trusted certs")
	}
	_, ok := ms.trustedCAs[proxyHost]
	if !ok {
		ms.trustedCAs[proxyHost] = map[string]*auth.TrustedCerts{}
	}
	for _, ca := range cas {
		if ca.ClusterName == "" {
			return trace.BadParameter("ca entry cannot have an empty cluster name")
		}
		_, ok := ms.trustedCAs[proxyHost][ca.ClusterName]
		if !ok {
			ms.trustedCAs[proxyHost][ca.ClusterName] = &auth.TrustedCerts{ClusterName: ca.ClusterName}
		}
		entry := ms.trustedCAs[proxyHost][ca.ClusterName]
		entry.TLSCertificates = append(entry.TLSCertificates, ca.TLSCertificates...)
		entry.HostCertificates = append(entry.HostCertificates, ca.HostCertificates...)
	}

	return nil
}

// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
func (ms *MemClientStore) GetTrustedCerts(proxyHost string) ([]auth.TrustedCerts, error) {
	var trustedCerts []auth.TrustedCerts
	for _, entry := range ms.trustedCAs[proxyHost] {
		trustedCerts = append(trustedCerts, *entry)
	}
	return trustedCerts, nil
}

// GetTrustedCertsPEM gets trusted TLS certificates of certificate authorities.
// Each returned byte slice contains an individual PEM block.
func (ms *MemClientStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	var tlsHostCerts [][]byte
	for _, ca := range ms.trustedCAs[proxyHost] {
		tlsHostCerts = append(tlsHostCerts, ca.TLSCertificates...)
	}
	return tlsHostCerts, nil
}

// GetKnownHostKeys returns all known public host keys for the given host name.
func (ms *MemClientStore) GetKnownHostKeys(clusterNames ...string) ([]ssh.PublicKey, error) {
	// known hosts are not retrieved by proxyHost, only clusterName, so we search all proxy entries.
	var hostKeys []ssh.PublicKey
	for _, proxyEntries := range ms.trustedCAs {
		for _, clusterName := range clusterNames {
			if entry, ok := proxyEntries[clusterName]; ok {
				clusterHostKeys, err := apisshutils.ParseAuthorizedKeys(entry.HostCertificates)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				hostKeys = append(hostKeys, clusterHostKeys...)
			}
		}
	}

	return hostKeys, nil
}

// CurrentProfile returns the current active profile.
func (ms *MemClientStore) CurrentProfile() (string, error) {
	return ms.currentProfile, nil
}

// ListProfiles returns a list of all active profiles.
func (ms *MemClientStore) ListProfiles() ([]string, error) {
	var profileNames []string
	for profileName := range ms.profiles {
		profileNames = append(profileNames, profileName)
	}
	return profileNames, nil
}

// GetProfile returns the requested profile.
func (ms *MemClientStore) GetProfile(profileName string) (*profile.Profile, error) {
	if profile, ok := ms.profiles[profileName]; ok {
		return profile, nil
	}
	return nil, trace.NotFound("profile for proxy host %q not found", profileName)
}

// SaveProfile saves the given profile
func (ms *MemClientStore) SaveProfile(profile *profile.Profile, makecurrent bool) error {
	ms.profiles[profile.Name()] = profile
	if makecurrent {
		ms.currentProfile = profile.Name()
	}
	return nil
}

// NewClientStoreFromIdentityFile creates a new local client store using the given identity file path.
func NewClientStoreFromIdentityFile(identityFile, proxyAddr, clusterName string) (ClientStore, error) {
	key, err := KeyFromIdentityFile(identityFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyHost, err := utils.Host(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key.ProxyHost = proxyHost
	if clusterName != "" {
		key.ClusterName = clusterName
	}

	keyStore := NewMemClientStore()

	// Save the temporary profile into the key store.
	profile := &profile.Profile{
		WebProxyAddr: proxyAddr,
		SiteName:     key.ClusterName,
		Username:     key.Username,
	}
	if err := keyStore.SaveProfile(profile, true); err != nil {
		return nil, trace.Wrap(err)
	}

	// Preload the client key from the agent.
	key.KeyIndex = KeyIndex{
		ProxyHost:   proxyHost,
		ClusterName: key.ClusterName,
		Username:    key.Username,
	}
	if err := keyStore.AddKey(key); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := keyStore.AddTrustedCerts(key.ProxyHost, key.TrustedCerts); err != nil {
		return nil, trace.Wrap(err)
	}

	return keyStore, nil
}

// addTrustedHostKeys is a helper function to add ssh host keys directly, rather than through AddTrustedCerts.
func addTrustedHostKeys(cs ClientStore, proxyHost string, clusterName string, hostKeys ...ssh.PublicKey) error {
	var authorizedKeys [][]byte
	for _, hostKey := range hostKeys {
		authorizedKeys = append(authorizedKeys, ssh.MarshalAuthorizedKey(hostKey))
	}
	err := cs.AddTrustedCerts(proxyHost, []auth.TrustedCerts{
		{
			ClusterName:      clusterName,
			HostCertificates: authorizedKeys,
		},
	})
	return trace.Wrap(err)
}
