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

package client

import (
	"errors"
	iofs "io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
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

// KeyStore is a storage interface for client session keys and certificates.
type KeyStore interface {
	// AddKey adds the given key to the store.
	AddKey(key *KeyRing) error

	// GetKey returns the user's key including the specified certs. The key's
	// TrustedCerts will be nil and should be filled in using a TrustedCertsStore.
	GetKey(idx KeyIndex, opts ...CertOption) (*KeyRing, error)

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

// FSKeyStore is an on-disk implementation of the KeyStore interface.
//
// The FS store uses the file layout outlined in `api/utils/keypaths.go`.
type FSKeyStore struct {
	// log holds the structured logger.
	log logrus.FieldLogger

	// KeyDir is the directory where all keys are stored.
	KeyDir string
}

// NewFSKeyStore initializes a new FSClientStore.
//
// If dirPath is empty, sets it to ~/.tsh.
func NewFSKeyStore(dirPath string) *FSKeyStore {
	dirPath = profile.FullProfilePath(dirPath)
	return &FSKeyStore{
		log:    logrus.WithField(teleport.ComponentKey, teleport.ComponentKeyStore),
		KeyDir: dirPath,
	}
}

// userKeyPath returns the private key path for the given KeyIndex.
func (fs *FSKeyStore) userKeyPath(idx KeyIndex) string {
	return keypaths.UserKeyPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// tlsCertPath returns the TLS certificate path given KeyIndex.
func (fs *FSKeyStore) tlsCertPath(idx KeyIndex) string {
	return keypaths.TLSCertPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// sshDir returns the SSH certificate path for the given KeyIndex.
func (fs *FSKeyStore) sshDir(proxy, user string) string {
	return keypaths.SSHDir(fs.KeyDir, proxy, user)
}

// sshCertPath returns the SSH certificate path for the given KeyIndex.
func (fs *FSKeyStore) sshCertPath(idx KeyIndex) string {
	return keypaths.SSHCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
}

// ppkFilePath returns the PPK (PuTTY-formatted) keypair path for the given KeyIndex.
func (fs *FSKeyStore) ppkFilePath(idx KeyIndex) string {
	return keypaths.PPKFilePath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// kubeCredLockfilePath returns kube credentials lockfile path for the given KeyIndex.
func (fs *FSKeyStore) kubeCredLockfilePath(idx KeyIndex) string {
	return keypaths.KubeCredLockfilePath(fs.KeyDir, idx.ProxyHost)
}

// publicKeyPath returns the public key path for the given KeyIndex.
func (fs *FSKeyStore) publicKeyPath(idx KeyIndex) string {
	return keypaths.PublicKeyPath(fs.KeyDir, idx.ProxyHost, idx.Username)
}

// appCertPath returns the TLS certificate path for the given KeyIndex and app name.
func (fs *FSKeyStore) appCertPath(idx KeyIndex, appname string) string {
	return keypaths.AppCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, appname)
}

// appKeyPath returns the private key path for the given KeyIndex and app name.
func (fs *FSKeyStore) appKeyPath(idx KeyIndex, appname string) string {
	return keypaths.AppKeyPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, appname)
}

// databaseCertPath returns the TLS certificate path for the given KeyIndex and database name.
func (fs *FSKeyStore) databaseCertPath(idx KeyIndex, dbname string) string {
	return keypaths.DatabaseCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, dbname)
}

// kubeCertPath returns the TLS certificate path for the given KeyIndex and kube cluster name.
func (fs *FSKeyStore) kubeCertPath(idx KeyIndex, kubename string) string {
	return keypaths.KubeCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, kubename)
}

// AddKey adds the given key to the store.
func (fs *FSKeyStore) AddKey(key *KeyRing) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}

	if err := fs.writeBytes(key.PrivateKey.PrivateKeyPEM(), fs.userKeyPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}

	if err := fs.writeBytes(key.PrivateKey.MarshalSSHPublicKey(), fs.publicKeyPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}

	// Store TLS cert
	if err := fs.writeBytes(key.TLSCert, fs.tlsCertPath(key.KeyIndex)); err != nil {
		return trace.Wrap(err)
	}

	// We only generate PPK files for use by PuTTY when running tsh on Windows.
	if runtime.GOOS == constants.WindowsOS {
		ppkFile, err := key.PrivateKey.PPKFile()
		// PPKFile can only be generated from an RSA private key. If the key is in a different
		// format, a BadParameter error is returned and we can skip PPK generation.
		if err != nil && !trace.IsBadParameter(err) {
			fs.log.Debugf("Cannot convert private key to PPK-formatted keypair: %v", err)
		} else {
			if err := fs.writeBytes(ppkFile, fs.ppkFilePath(key.KeyIndex)); err != nil {
				return trace.Wrap(err)
			}
		}
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
	for app, cred := range key.AppTLSCredentials {
		app = filepath.Clean(app)
		certPath := fs.appCertPath(key.KeyIndex, app)
		if err := fs.writeBytes(cred.Cert, certPath); err != nil {
			return trace.Wrap(err)
		}
		keyPath := fs.appKeyPath(key.KeyIndex, app)
		if err := fs.writeBytes(cred.PrivateKey.PrivateKeyPEM(), keyPath); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (fs *FSKeyStore) writeBytes(bytes []byte, fp string) error {
	if err := os.MkdirAll(filepath.Dir(fp), os.ModeDir|profileDirPerms); err != nil {
		return trace.ConvertSystemError(err)
	}
	err := os.WriteFile(fp, bytes, keyFilePerms)
	return trace.ConvertSystemError(err)
}

// DeleteKey deletes the user's key with all its certs.
func (fs *FSKeyStore) DeleteKey(idx KeyIndex) error {
	files := []string{
		fs.userKeyPath(idx),
		fs.publicKeyPath(idx),
		fs.tlsCertPath(idx),
	}
	for _, fn := range files {
		if err := utils.RemoveSecure(fn); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	// we also need to delete the extra PuTTY-formatted .ppk file when running on Windows,
	// but it may not exist when upgrading from v9 -> v10 and logging into an existing cluster.
	// as such, deletion should be best-effort and not generate an error if it fails.
	if runtime.GOOS == constants.WindowsOS {
		_ = utils.RemoveSecure(fs.ppkFilePath(idx))
	}

	// And try to delete kube credentials lockfile in case it exists
	err := utils.RemoveSecure(fs.kubeCredLockfilePath(idx))
	if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		log.Debugf("Could not remove kube credentials file: %v", err)
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
func (fs *FSKeyStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	var pathsToDelete []string
	for _, o := range opts {
		pathsToDelete = append(pathsToDelete, o.pathsToDelete(fs.KeyDir, idx)...)
	}
	for _, path := range pathsToDelete {
		if err := utils.RemoveAllSecure(path); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// DeleteKeys removes all session keys.
func (fs *FSKeyStore) DeleteKeys() error {
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
			err := utils.RemoveAllSecure(filepath.Join(fs.KeyDir, file.Name()))
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			continue
		}
		err := utils.RemoveAllSecure(filepath.Join(fs.KeyDir, file.Name()))
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// GetKey returns the user's key including the specified certs.
// If the key is not found, returns trace.NotFound error.
func (fs *FSKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*KeyRing, error) {
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
		return nil, trace.ConvertSystemError(err)
	}

	priv, err := keys.LoadKeyPair(fs.userKeyPath(idx), fs.publicKeyPath(idx))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	key := NewKeyRing(priv)
	key.KeyIndex = idx
	key.TLSCert = tlsCert

	for _, o := range opts {
		if err := fs.updateKeyRingWithCerts(o, key); err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	// Note, we may be returning expired certificates here, that is okay. If a
	// certificate is expired, it's the responsibility of the TeleportClient to
	// perform cleanup of the certificates and the profile.

	return key, nil
}

func (fs *FSKeyStore) updateKeyRingWithCerts(o CertOption, keyRing *KeyRing) error {
	return trace.Wrap(o.updateKeyRing(fs.KeyDir, keyRing.KeyIndex, keyRing))
}

// GetSSHCertificates gets all certificates signed for the given user and proxy.
func (fs *FSKeyStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
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

func getCertsByName(certDir string) (map[string][]byte, error) {
	certsByName := make(map[string][]byte)
	certFiles, err := os.ReadDir(certDir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	for _, certFile := range certFiles {
		name := keypaths.TrimCertPathSuffix(certFile.Name())
		if isCert := name != certFile.Name(); isCert {
			data, err := os.ReadFile(filepath.Join(certDir, certFile.Name()))
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}
			certsByName[name] = data
		}
	}
	return certsByName, nil
}

func getCredentialsByName(credentialDir string) (map[string]TLSCredential, error) {
	credsByName := make(map[string]TLSCredential)
	files, err := os.ReadDir(credentialDir)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	for _, file := range files {
		if certName := keypaths.TrimCertPathSuffix(file.Name()); certName != file.Name() {
			data, err := os.ReadFile(filepath.Join(credentialDir, file.Name()))
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}
			cred := credsByName[certName]
			cred.Cert = data
			credsByName[certName] = cred
		}
		if keyName := keypaths.TrimKeyPathSuffix(file.Name()); keyName != file.Name() {
			data, err := os.ReadFile(filepath.Join(credentialDir, file.Name()))
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}
			privKey, err := keys.ParsePrivateKey(data)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			cred := credsByName[keyName]
			cred.PrivateKey = privKey
			credsByName[keyName] = cred
		}
	}
	for name, creds := range credsByName {
		if creds.Cert == nil || creds.PrivateKey == nil {
			// Found a cert with no key or vice-versa, this may have been
			// written by an older version or partially deleted, treat it as
			// missing and a cert re-issue should solve it.
			delete(credsByName, name)
		}
	}
	return credsByName, nil
}

// CertOption is an additional step to run when loading/deleting user certificates.
type CertOption interface {
	// updateKeyRing is used by [FSKeyStore] to add the relevant credentials
	// loaded from disk to [keyRing].
	updateKeyRing(keyDir string, idx KeyIndex, keyRing *KeyRing) error
	// pathsToDelete is used by [FSKeyStore] to get all the paths (files and/or
	// directories) that should be deleted by [DeleteUserCerts].
	pathsToDelete(keyDir string, idx KeyIndex) []string
	// deleteFromKeyRing deletes the credential data from the [KeyRing].
	deleteFromKeyRing(*KeyRing)
}

// WithAllCerts lists all known CertOptions.
var WithAllCerts = []CertOption{WithSSHCerts{}, WithKubeCerts{}, WithDBCerts{}, WithAppCerts{}}

// WithSSHCerts is a CertOption for handling SSH certificates.
type WithSSHCerts struct{}

func (o WithSSHCerts) updateKeyRing(keyDir string, idx KeyIndex, keyRing *KeyRing) error {
	certPath := keypaths.SSHCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	cert, err := os.ReadFile(certPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	keyRing.Cert = cert
	return nil
}

func (o WithSSHCerts) pathsToDelete(keyDir string, idx KeyIndex) []string {
	if idx.ClusterName == "" {
		return []string{keypaths.SSHDir(keyDir, idx.ProxyHost, idx.Username)}
	}
	return []string{keypaths.SSHCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)}
}

func (o WithSSHCerts) deleteFromKeyRing(key *KeyRing) {
	key.Cert = nil
}

// WithKubeCerts is a CertOption for handling kubernetes certificates.
type WithKubeCerts struct{}

func (o WithKubeCerts) updateKeyRing(keyDir string, idx KeyIndex, keyRing *KeyRing) error {
	certDir := keypaths.KubeCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	certsByName, err := getCertsByName(certDir)
	if err != nil {
		return trace.Wrap(err)
	}
	keyRing.KubeTLSCerts = certsByName
	return nil
}

func (o WithKubeCerts) pathsToDelete(keyDir string, idx KeyIndex) []string {
	if idx.ClusterName == "" {
		return []string{keypaths.KubeDir(keyDir, idx.ProxyHost, idx.Username)}
	}
	return []string{keypaths.KubeCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)}
}

func (o WithKubeCerts) deleteFromKeyRing(key *KeyRing) {
	key.KubeTLSCerts = make(map[string][]byte)
}

// WithDBCerts is a CertOption for handling database access certificates.
type WithDBCerts struct {
	dbName string
}

func (o WithDBCerts) updateKeyRing(keyDir string, idx KeyIndex, keyRing *KeyRing) error {
	certDir := keypaths.DatabaseCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	certsByName, err := getCertsByName(certDir)
	if err != nil {
		return trace.Wrap(err)
	}
	keyRing.DBTLSCerts = certsByName
	return nil
}

func (o WithDBCerts) pathsToDelete(keyDir string, idx KeyIndex) []string {
	if idx.ClusterName == "" {
		return []string{keypaths.DatabaseDir(keyDir, idx.ProxyHost, idx.Username)}
	}
	if o.dbName == "" {
		return []string{keypaths.DatabaseCertDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)}
	}
	return []string{keypaths.DatabaseCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName, o.dbName)}
}

func (o WithDBCerts) deleteFromKeyRing(key *KeyRing) {
	key.DBTLSCerts = make(map[string][]byte)
}

// WithAppCerts is a CertOption for handling application access certificates.
type WithAppCerts struct {
	appName string
}

func (o WithAppCerts) updateKeyRing(keyDir string, idx KeyIndex, keyRing *KeyRing) error {
	credentialDir := keypaths.AppCredentialDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)
	credsByName, err := getCredentialsByName(credentialDir)
	if err != nil {
		return trace.Wrap(err)
	}
	keyRing.AppTLSCredentials = credsByName
	return nil
}

func (o WithAppCerts) pathsToDelete(keyDir string, idx KeyIndex) []string {
	if idx.ClusterName == "" {
		return []string{keypaths.AppDir(keyDir, idx.ProxyHost, idx.Username)}
	}
	if o.appName == "" {
		return []string{keypaths.AppCredentialDir(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName)}
	}
	return []string{
		keypaths.AppCertPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName, o.appName),
		keypaths.AppKeyPath(keyDir, idx.ProxyHost, idx.Username, idx.ClusterName, o.appName),
	}
}

func (o WithAppCerts) deleteFromKeyRing(key *KeyRing) {
	key.AppTLSCredentials = make(map[string]TLSCredential)
}

type MemKeyStore struct {
	// keys is a three-dimensional map indexed by [proxyHost][username][clusterName]
	keys keyMap
}

// keyMap is a three-dimensional map indexed by [proxyHost][username][clusterName]
type keyMap map[string]map[string]map[string]*KeyRing

func NewMemKeyStore() *MemKeyStore {
	return &MemKeyStore{
		keys: make(keyMap),
	}
}

// AddKey writes a key to the underlying key store.
func (ms *MemKeyStore) AddKey(key *KeyRing) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, ok := ms.keys[key.ProxyHost]
	if !ok {
		ms.keys[key.ProxyHost] = map[string]map[string]*KeyRing{}
	}
	_, ok = ms.keys[key.ProxyHost][key.Username]
	if !ok {
		ms.keys[key.ProxyHost][key.Username] = map[string]*KeyRing{}
	}
	keyCopy := key.Copy()

	// TrustedCA is stored separately in the Memory store so we wipe out
	// the keys' trusted CA to prevent inconsistencies.
	keyCopy.TrustedCerts = nil

	ms.keys[key.ProxyHost][key.Username][key.ClusterName] = keyCopy

	return nil
}

// GetKey returns the user's key including the specified certs.
func (ms *MemKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*KeyRing, error) {
	if len(opts) > 0 {
		if err := idx.Check(); err != nil {
			return nil, trace.Wrap(err, "GetKey with CertOptions requires a fully specified KeyIndex")
		}
	}

	// If clusterName is not specified then the cluster-dependent fields
	// are not considered relevant and we may simply return any key
	// associated with any cluster name whatsoever.
	var key *KeyRing
	if idx.ClusterName == "" {
		for _, k := range ms.keys[idx.ProxyHost][idx.Username] {
			key = k
			break
		}
	} else {
		if k, ok := ms.keys[idx.ProxyHost][idx.Username][idx.ClusterName]; ok {
			key = k
		}
	}

	if key == nil {
		return nil, trace.NotFound("key for %+v not found", idx)
	}

	retKey := NewKeyRing(key.PrivateKey)
	retKey.KeyIndex = idx
	retKey.TLSCert = key.TLSCert
	for _, o := range opts {
		switch o.(type) {
		case WithSSHCerts:
			retKey.Cert = key.Cert
		case WithKubeCerts:
			retKey.KubeTLSCerts = key.KubeTLSCerts
		case WithDBCerts:
			retKey.DBTLSCerts = key.DBTLSCerts
		case WithAppCerts:
			retKey.AppTLSCredentials = key.AppTLSCredentials
		}
	}

	return retKey, nil
}

// DeleteKey deletes the user's key with all its certs.
func (ms *MemKeyStore) DeleteKey(idx KeyIndex) error {
	if _, ok := ms.keys[idx.ProxyHost][idx.Username][idx.ClusterName]; !ok {
		return trace.NotFound("key for %+v not found", idx)
	}
	delete(ms.keys[idx.ProxyHost], idx.Username)
	return nil
}

// DeleteKeys removes all session keys.
func (ms *MemKeyStore) DeleteKeys() error {
	ms.keys = make(keyMap)
	return nil
}

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
// Empty clusterName indicates to delete the certs for all clusters.
//
// Useful when needing to log out of a specific service, like a particular
// database proxy.
func (ms *MemKeyStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	var keys []*KeyRing
	if idx.ClusterName != "" {
		key, ok := ms.keys[idx.ProxyHost][idx.Username][idx.ClusterName]
		if !ok {
			return nil
		}
		keys = []*KeyRing{key}
	} else {
		keys = make([]*KeyRing, 0, len(ms.keys[idx.ProxyHost][idx.Username]))
		for _, key := range ms.keys[idx.ProxyHost][idx.Username] {
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		for _, o := range opts {
			o.deleteFromKeyRing(key)
		}
	}
	return nil
}

// GetSSHCertificates gets all certificates signed for the given user and proxy.
func (ms *MemKeyStore) GetSSHCertificates(proxyHost, username string) ([]*ssh.Certificate, error) {
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
