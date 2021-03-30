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
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

const (
	kubeDirSuffix = "-kube"
	dbDirSuffix   = "-db"
	appDirSuffix  = "-app"

	// profileDirPerms is the default permissions applied to the profile
	// directory (usually ~/.tsh)
	profileDirPerms os.FileMode = 0700

	// keyFilePerms is the default permissions applied to key files (.cert, .key, pub)
	// under ~/.tsh
	keyFilePerms os.FileMode = 0600
)

// LocalKeyStore interface allows for different storage backends for tsh to
// load/save its keys.
//
// The _only_ filesystem-based implementation of LocalKeyStore is declared
// below (FSLocalKeyStore)
type LocalKeyStore interface {
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

	// AddKnownHostKeys adds the public key to the list of known hosts for
	// a hostname.
	AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error

	// GetKnownHostKeys returns all public keys for a hostname.
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)

	// SaveTrustedCerts saves trusted TLS certificates of certificate authorities.
	SaveTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error

	// GetTrustedCertsPEM gets trusted TLS certificates of certificate authorities.
	// Each returned byte slice contains an individual PEM block.
	GetTrustedCertsPEM(proxyHost string) ([][]byte, error)
}

// FSLocalKeyStore implements LocalKeyStore interface using the filesystem.
// Here's the file layout for the FS store:
//
// ~/.tsh/
// ├── known_hosts                   --> trusted certificate authorities (their keys) in a format similar to known_hosts
// └── keys
//    ├── one.example.com            --> Proxy hostname
//    │   ├── certs.pem              --> TLS CA certs for the Teleport CA
//    │   ├── foo                    --> RSA Private Key for user "foo"
//    │   ├── foo.pub                --> Public Key
//    │   ├── foo-x509.pem           --> TLS client certificate for Auth Server
//    │   ├── foo-ssh                --> SSH certs for user "foo"
//    │   │   ├── root-cert.pub      --> SSH cert for Teleport cluster "root"
//    │   │   └── leaf-cert.pub      --> SSH cert for Teleport cluster "leaf"
//    │   ├── foo-kube               --> Kubernetes certs for user "foo"
//    │   │   ├── root               --> Kubernetes certs for Teleport cluster "root"
//    │   │   │   ├── kubeA-x509.pem --> TLS cert for Kubernetes cluster "kubeA"
//    │   │   │   └── kubeB-x509.pem --> TLS cert for Kubernetes cluster "kubeB"
//    │   │   └── leaf               --> Kubernetes certs for Teleport cluster "leaf"
//    │   │       └── kubeC-x509.pem --> TLS cert for Kubernetes cluster "kubeC"
//    │   └── foo-db                 --> Database access certs for user "foo"
//    │       ├── root               --> Database access certs for cluster "root"
//    │       │   ├── dbA-x509.pem   --> TLS cert for database service "dbA"
//    │       │   └── dbB-x509.pem   --> TLS cert for database service "dbB"
//    │       └── leaf               --> Database access certs for cluster "leaf"
//    │           └── dbC-x509.pem   --> TLS cert for database service "dbC"
//    └── two.example.com
//        ├── certs.pem
//        ├── bar
//        ├── bar.pub
//        ├── bar-x509.pem
//        └── bar-ssh
//            └── clusterA-cert.pub
type FSLocalKeyStore struct {
	fsLocalNonSessionKeyStore
}

// NewFSLocalKeyStore creates a new filesystem-based local keystore object
// and initializes it.
//
// If dirPath is empty, sets it to ~/.tsh.
func NewFSLocalKeyStore(dirPath string) (s *FSLocalKeyStore, err error) {
	dirPath, err = initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FSLocalKeyStore{
		fsLocalNonSessionKeyStore: fsLocalNonSessionKeyStore{
			log:    logrus.WithField(trace.Component, teleport.ComponentKeyStore),
			KeyDir: dirPath,
		},
	}, nil
}

// initKeysDir initializes the keystore root directory, usually `~/.tsh`.
func initKeysDir(dirPath string) (string, error) {
	dirPath = client.FullProfilePath(dirPath)
	if err := os.MkdirAll(dirPath, os.ModeDir|profileDirPerms); err != nil {
		return "", trace.ConvertSystemError(err)
	}
	return dirPath, nil
}

// AddKey adds the given key to the store.
func (fs *FSLocalKeyStore) AddKey(key *Key) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}

	inProxyHostDir := func(path ...string) string {
		return fs.inSessionKeyDir(key.ProxyHost, filepath.Join(path...))
	}

	// Store core key data.
	if err := fs.writeBytes(key.Priv, inProxyHostDir(key.Username)); err != nil {
		return trace.Wrap(err)
	}
	if err := fs.writeBytes(key.Pub, inProxyHostDir(key.Username+constants.FileExtPub)); err != nil {
		return trace.Wrap(err)
	}
	if err := fs.writeBytes(key.TLSCert, inProxyHostDir(key.Username+constants.FileExtTLSCert)); err != nil {
		return trace.Wrap(err)
	}

	// Store per-cluster key data.
	if err := fs.writeBytes(key.Cert, inProxyHostDir(key.Username+constants.SSHDirSuffix, key.ClusterName+constants.FileExtSSHCert)); err != nil {
		return trace.Wrap(err)
	}
	// TODO(awly): unit test this.
	for kubeCluster, cert := range key.KubeTLSCerts {
		// Prevent directory traversal via a crafted kubernetes cluster name.
		//
		// This will confuse cluster cert loading (GetKey will return
		// kubernetes cluster names different from the ones stored here), but I
		// don't expect any well-meaning user to create bad names.
		kubeCluster = filepath.Clean(kubeCluster)

		path := inProxyHostDir(key.Username+kubeDirSuffix, key.ClusterName, kubeCluster+constants.FileExtTLSCert)
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}
	for db, cert := range key.DBTLSCerts {
		path := inProxyHostDir(key.Username+dbDirSuffix, key.ClusterName, filepath.Clean(db)+constants.FileExtTLSCert)
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}
	for app, cert := range key.AppTLSCerts {
		path := inProxyHostDir(key.Username+appDirSuffix, key.ClusterName, filepath.Clean(app)+constants.FileExtTLSCert)
		if err := fs.writeBytes(cert, path); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (fs *FSLocalKeyStore) writeBytes(bytes []byte, fp string) error {
	if err := os.MkdirAll(filepath.Dir(fp), os.ModeDir|profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}
	err := ioutil.WriteFile(fp, bytes, keyFilePerms)
	if err != nil {
		fs.log.Error(err)
	}
	return trace.ConvertSystemError(err)
}

// DeleteKey deletes the user's key with all its certs.
func (fs *FSLocalKeyStore) DeleteKey(idx KeyIndex) error {
	files := []string{
		fs.inSessionKeyDir(idx.ProxyHost, idx.Username),
		fs.inSessionKeyDir(idx.ProxyHost, idx.Username+constants.FileExtPub),
		fs.inSessionKeyDir(idx.ProxyHost, idx.Username+constants.FileExtTLSCert),
	}
	for _, fn := range files {
		if err := os.Remove(fn); err != nil {
			return trace.ConvertSystemError(err)
		}
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
func (fs *FSLocalKeyStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	for _, o := range opts {
		certPath := fs.inSessionKeyDir(o.relativeCertPath(idx))
		if err := os.RemoveAll(certPath); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	return nil
}

// DeleteKeys removes all session keys.
func (fs *FSLocalKeyStore) DeleteKeys() error {
	if err := os.RemoveAll(fs.inSessionKeyDir()); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// GetKey returns the user's key including the specified certs.
// If the key is not found, returns trace.NotFound error.
func (fs *FSLocalKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	if len(opts) > 0 {
		if err := idx.Check(); err != nil {
			return nil, trace.Wrap(err, "GetKey with CertOptions requires a fully specified KeyIndex")
		}
	}

	if _, err := ioutil.ReadDir(fs.inSessionKeyDir()); err != nil && trace.IsNotFound(err) {
		return nil, trace.Wrap(err, "no session keys for %+v", idx)
	}

	priv, err := ioutil.ReadFile(fs.inSessionKeyDir(idx.ProxyHost, idx.Username))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}
	pub, err := ioutil.ReadFile(fs.inSessionKeyDir(idx.ProxyHost, idx.Username+constants.FileExtPub))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}
	tlsCertFile := fs.inSessionKeyDir(idx.ProxyHost, idx.Username+constants.FileExtTLSCert)
	tlsCert, err := ioutil.ReadFile(tlsCertFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}
	tlsCA, err := fs.GetTrustedCertsPEM(idx.ProxyHost)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.ConvertSystemError(err)
	}

	key := &Key{
		KeyIndex: idx,
		Pub:      pub,
		Priv:     priv,
		TLSCert:  tlsCert,
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: tlsCA,
		}},
		KubeTLSCerts: make(map[string][]byte),
		DBTLSCerts:   make(map[string][]byte),
		AppTLSCerts:  make(map[string][]byte),
	}

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

func (fs *FSLocalKeyStore) updateKeyWithCerts(o CertOption, key *Key) error {
	certPath := fs.inSessionKeyDir(o.relativeCertPath(key.KeyIndex))
	info, err := os.Stat(certPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	fs.log.Debugf("Reading certificates from path %q.", certPath)

	if info.IsDir() {
		certDataMap := map[string][]byte{}
		certFiles, err := ioutil.ReadDir(certPath)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		for _, certFile := range certFiles {
			data, err := ioutil.ReadFile(filepath.Join(certPath, certFile.Name()))
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			name := strings.TrimSuffix(certFile.Name(), constants.FileExtTLSCert)
			certDataMap[name] = data
		}
		return o.updateKeyWithMap(key, certDataMap)
	}

	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	return o.updateKeyWithBytes(key, certBytes)
}

// CertOption is an additional step to run when loading/deleting user certificates.
type CertOption interface {
	// relativeCertPath returns a path to the cert (or to a dir holding the certs)
	// relative to the session key dir. For use with FSLocalKeyStore.
	relativeCertPath(idx KeyIndex) string
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

func (o WithSSHCerts) relativeCertPath(idx KeyIndex) string {
	components := []string{idx.ProxyHost, idx.Username + constants.SSHDirSuffix}
	if idx.ClusterName != "" {
		components = append(components, idx.ClusterName+constants.FileExtSSHCert)
	}
	return filepath.Join(components...)
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

func (o WithKubeCerts) relativeCertPath(idx KeyIndex) string {
	components := []string{idx.ProxyHost, idx.Username + kubeDirSuffix}
	if idx.ClusterName != "" {
		components = append(components, idx.ClusterName)
	}
	return filepath.Join(components...)
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

func (o WithDBCerts) relativeCertPath(idx KeyIndex) string {
	components := []string{idx.ProxyHost, idx.Username + dbDirSuffix}
	if idx.ClusterName != "" {
		components = append(components, idx.ClusterName)
		if o.dbName != "" {
			components = append(components, o.dbName+constants.FileExtTLSCert)
		}
	}
	return filepath.Join(components...)
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

func (o WithAppCerts) relativeCertPath(idx KeyIndex) string {
	components := []string{idx.ProxyHost, idx.Username + appDirSuffix}
	if idx.ClusterName != "" {
		components = append(components, idx.ClusterName)
		if o.appName != "" {
			components = append(components, o.appName+constants.FileExtTLSCert)
		}
	}
	return filepath.Join(components...)
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

// fsLocalNonSessionKeyStore is a FS-based store implementing methods
// for CA certificates and known host fingerprints. It is embedded
// in both FSLocalKeyStore and MemLocalKeyStore.
type fsLocalNonSessionKeyStore struct {
	// log holds the structured logger.
	log logrus.FieldLogger

	// KeyDir is the directory where all keys are stored.
	KeyDir string
}

// inSessionKeyDir prepends the given path components with the session key dir,
// usually returning a path of the form `~/.tsh/keys/<path0>/<path1>/<...>`.
func (fs *fsLocalNonSessionKeyStore) inSessionKeyDir(path ...string) string {
	return filepath.Join(fs.KeyDir, constants.SessionKeyDir, filepath.Join(path...))
}

// AddKnownHostKeys adds a new entry to `known_hosts` file.
func (fs *fsLocalNonSessionKeyStore) AddKnownHostKeys(hostname string, hostKeys []ssh.PublicKey) (retErr error) {
	fp, err := os.OpenFile(filepath.Join(fs.KeyDir, constants.FileNameKnownHosts), os.O_CREATE|os.O_RDWR, 0640)
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
	for i := range hostKeys {
		fs.log.Debugf("Adding known host %s with key: %v", hostname, sshutils.Fingerprint(hostKeys[i]))
		bytes := ssh.MarshalAuthorizedKey(hostKeys[i])
		line := strings.TrimSpace(fmt.Sprintf("%s %s", hostname, bytes))
		if _, exists := entries[line]; !exists {
			output = append(output, line)
		}
	}
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

// GetKnownHostKeys returns all known public keys from `known_hosts`.
func (fs *fsLocalNonSessionKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(fs.KeyDir, constants.FileNameKnownHosts))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	var (
		pubKey    ssh.PublicKey
		retval    []ssh.PublicKey = make([]ssh.PublicKey, 0)
		hosts     []string
		hostMatch bool
	)
	for err == nil {
		_, hosts, pubKey, _, bytes, err = ssh.ParseKnownHosts(bytes)
		if err == nil {
			hostMatch = (hostname == "")
			if !hostMatch {
				for i := range hosts {
					if hosts[i] == hostname {
						hostMatch = true
						break
					}
				}
			}
			if hostMatch {
				retval = append(retval, pubKey)
			}
		}
	}
	if err != io.EOF {
		return nil, trace.Wrap(err)
	}
	return retval, nil
}

// SaveTrustedCerts saves trusted TLS certificates of certificate authorities.
func (fs *fsLocalNonSessionKeyStore) SaveTrustedCerts(proxyHost string, cas []auth.TrustedCerts) (retErr error) {
	if err := os.MkdirAll(fs.inSessionKeyDir(proxyHost), os.ModeDir|profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}
	certsFile := fs.inSessionKeyDir(proxyHost, constants.FileNameTLSCerts)
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

// GetTrustedCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (fs *fsLocalNonSessionKeyStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	data, err := ioutil.ReadFile(fs.inSessionKeyDir(proxyHost, constants.FileNameTLSCerts))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var blocks [][]byte
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			fs.log.Debugf("Skipping PEM block type=%v headers=%v.", block.Type, block.Headers)
			continue
		}
		// rest contains the remainder of data after reading a block.
		// Therefore, the block length is len(data) - len(rest).
		// Use that length to slice the block from the start of data.
		blocks = append(blocks, data[:len(data)-len(rest)])
		data = rest
	}
	return blocks, nil
}

// noLocalKeyStore is a LocalKeyStore representing the absence of a keystore.
// All methods return errors. This exists to avoid nil checking everywhere in
// LocalKeyAgent and prevent nil pointer panics.
type noLocalKeyStore struct{}

var errNoLocalKeyStore = trace.NotFound("there is no local keystore")

func (noLocalKeyStore) AddKey(key *Key) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	return nil, errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteKey(idx KeyIndex) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteKeys() error { return errNoLocalKeyStore }
func (noLocalKeyStore) AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
	return nil, errNoLocalKeyStore
}
func (noLocalKeyStore) SaveTrustedCerts(proxyHost string, cas []auth.TrustedCerts) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	return nil, errNoLocalKeyStore
}

// MemLocalKeyStore is an in-memory session keystore implementation.
type MemLocalKeyStore struct {
	fsLocalNonSessionKeyStore
	inMem memLocalKeyStoreMap
}

// memLocalKeyStoreMap is a three-dimensional map indexed by [proxyHost][username][clusterName]
type memLocalKeyStoreMap = map[string]map[string]map[string]*Key

// NewMemLocalKeyStore initializes a MemLocalKeyStore.
// The key directory here is only used for storing CA certificates and known
// host fingerprints.
func NewMemLocalKeyStore(dirPath string) (*MemLocalKeyStore, error) {
	dirPath, err := initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &MemLocalKeyStore{
		fsLocalNonSessionKeyStore{
			log:    logrus.WithField(trace.Component, teleport.ComponentKeyStore),
			KeyDir: dirPath,
		},
		memLocalKeyStoreMap{},
	}, nil
}

// AddKey writes a key to the underlying key store.
func (s *MemLocalKeyStore) AddKey(key *Key) error {
	if err := key.KeyIndex.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, ok := s.inMem[key.ProxyHost]
	if !ok {
		s.inMem[key.ProxyHost] = map[string]map[string]*Key{}
	}
	_, ok = s.inMem[key.ProxyHost][key.Username]
	if !ok {
		s.inMem[key.ProxyHost][key.Username] = map[string]*Key{}
	}
	s.inMem[key.ProxyHost][key.Username][key.ClusterName] = key
	return nil
}

// GetKey returns the user's key including the specified certs.
func (s *MemLocalKeyStore) GetKey(idx KeyIndex, opts ...CertOption) (*Key, error) {
	var key *Key
	if idx.ClusterName == "" {
		// If clusterName is not specified then the cluster-dependent fields
		// are not considered relevant and we may simply return any key
		// associated with any cluster name whatsoever.
		for _, found := range s.inMem[idx.ProxyHost][idx.Username] {
			key = found
			break
		}
	} else {
		key = s.inMem[idx.ProxyHost][idx.Username][idx.ClusterName]
	}
	if key == nil {
		return nil, trace.NotFound("key for %+v not found", idx)
	}

	// It is not necessary to handle opts because all the optional certs are
	// already part of the Key struct as stored in memory.

	tlsCertExpiration, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Returning Teleport TLS certificate from memory, valid until %q.", tlsCertExpiration)

	// Validate the SSH certificate.
	if err := key.CheckCert(); err != nil {
		if !utils.IsCertExpiredError(err) {
			return nil, trace.Wrap(err)
		}
	}

	return key, nil
}

// DeleteKey deletes the user's key with all its certs.
func (s *MemLocalKeyStore) DeleteKey(idx KeyIndex) error {
	delete(s.inMem[idx.ProxyHost], idx.Username)
	return nil
}

// DeleteKeys removes all session keys.
func (s *MemLocalKeyStore) DeleteKeys() error {
	s.inMem = memLocalKeyStoreMap{}
	return nil
}

// DeleteUserCerts deletes only the specified certs of the user's key,
// keeping the private key intact.
// Empty clusterName indicates to delete the certs for all clusters.
//
// Useful when needing to log out of a specific service, like a particular
// database proxy.
func (s *MemLocalKeyStore) DeleteUserCerts(idx KeyIndex, opts ...CertOption) error {
	var keys []*Key
	if idx.ClusterName != "" {
		key, ok := s.inMem[idx.ProxyHost][idx.Username][idx.ClusterName]
		if !ok {
			return nil
		}
		keys = []*Key{key}
	} else {
		keys = make([]*Key, 0, len(s.inMem[idx.ProxyHost][idx.Username]))
		for _, key := range s.inMem[idx.ProxyHost][idx.Username] {
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
