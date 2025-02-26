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

	// tshBin is the name of the directory containing the
	// updated binaries of client tools.
	tshBin = "bin"
)

// KeyStore is a storage interface for client session keys and certificates.
type KeyStore interface {
	// AddKey adds the given key to the store.
	AddKey(key *Key) error

	// GetKey returns the user's key including the specified certs. The key's
	// TrustedCerts will be nil and should be filled in using a TrustedCertsStore.
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
		log:    logrus.WithField(trace.Component, teleport.ComponentKeyStore),
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

// databaseCertPath returns the TLS certificate path for the given KeyIndex and database name.
func (fs *FSKeyStore) databaseCertPath(idx KeyIndex, dbname string) string {
	return keypaths.DatabaseCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, dbname)
}

// kubeCertPath returns the TLS certificate path for the given KeyIndex and kube cluster name.
func (fs *FSKeyStore) kubeCertPath(idx KeyIndex, kubename string) string {
	return keypaths.KubeCertPath(fs.KeyDir, idx.ProxyHost, idx.Username, idx.ClusterName, kubename)
}

// AddKey adds the given key to the store.
func (fs *FSKeyStore) AddKey(key *Key) error {
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

	// We only generate PPK files for use by PuTTY when running tsh on Windows.
	if runtime.GOOS == constants.WindowsOS {
		ppkFile, err := key.PPKFile()
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
	for app, cert := range key.AppTLSCerts {
		path := fs.appCertPath(key.KeyIndex, filepath.Clean(app))
		if err := fs.writeBytes(cert, path); err != nil {
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
	for _, o := range opts {
		certPath := o.certPath(fs.KeyDir, idx)
		if err := utils.RemoveAllSecure(certPath); err != nil {
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
	ignoreDirs := map[string]struct{}{tshConfigFileName: {}, tshAzureDirName: {}, tshBin: {}}
	for _, file := range files {
		// Don't delete 'config', 'azure' and 'bin' directories.
		// TODO: this is hackish and really shouldn't be needed, but fs.KeyDir is `~/.tsh` while it probably should be `~/.tsh/keys` instead.
		if _, ok := ignoreDirs[file.Name()]; ok && file.IsDir() {
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
func (fs *FSKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
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

	key := NewKey(priv)
	key.KeyIndex = idx
	key.TLSCert = tlsCert

	for _, o := range opts {
		if err := fs.updateKeyWithCerts(o, key); err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}

	// Note, we may be returning expired certificates here, that is okay. If a
	// certificate is expired, it's the responsibility of the TeleportClient to
	// perform cleanup of the certificates and the profile.

	return key, nil
}

func (fs *FSKeyStore) updateKeyWithCerts(o CertOption, key *Key) error {
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
	key.KubeTLSCerts = make(map[string][]byte)
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
	key.DBTLSCerts = make(map[string][]byte)
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
	key.AppTLSCerts = make(map[string][]byte)
}

type MemKeyStore struct {
	// keys is a three-dimensional map indexed by [proxyHost][username][clusterName]
	keys keyMap
}

// keyMap is a three-dimensional map indexed by [proxyHost][username][clusterName]
type keyMap map[string]map[string]map[string]*Key

func NewMemKeyStore() *MemKeyStore {
	return &MemKeyStore{
		keys: make(keyMap),
	}
}

// AddKey writes a key to the underlying key store.
func (ms *MemKeyStore) AddKey(key *Key) error {
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
	keyCopy := key.Copy()

	// TrustedCA is stored separately in the Memory store so we wipe out
	// the keys' trusted CA to prevent inconsistencies.
	keyCopy.TrustedCerts = nil

	ms.keys[key.ProxyHost][key.Username][key.ClusterName] = keyCopy

	return nil
}

// GetKey returns the user's key including the specified certs.
func (ms *MemKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	if len(opts) > 0 {
		if err := idx.Check(); err != nil {
			return nil, trace.Wrap(err, "GetKey with CertOptions requires a fully specified KeyIndex")
		}
	}

	// If clusterName is not specified then the cluster-dependent fields
	// are not considered relevant and we may simply return any key
	// associated with any cluster name whatsoever.
	var key *Key
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

	retKey := NewKey(key.PrivateKey)
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
			retKey.AppTLSCerts = key.AppTLSCerts
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
