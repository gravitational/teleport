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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
)

const (
	defaultKeyDir      = ProfileDir
	fileExtTLSCert     = "-x509.pem"
	fileExtCert        = "-cert.pub"
	fileExtPub         = ".pub"
	sessionKeyDir      = "keys"
	fileNameKnownHosts = "known_hosts"
	fileNameTLSCerts   = "certs.pem"
	kubeDirSuffix      = "-kube"
	dbDirSuffix        = "-db"

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
//
// TODO: add `getKubeTLSCerts`, `deleteKubeTLSCerts`, etc methods to `LocalKeyStore` to avoid handling different keystore implementations inside `KeyOption` implementations.
type LocalKeyStore interface {
	// AddKey adds the given session key for the proxy and username to the
	// storage backend.
	AddKey(proxy string, username string, key *Key) error

	// GetKey returns the session key for the given username and proxy.
	GetKey(proxy, username string, opts ...KeyOption) (*Key, error)

	// DeleteKey removes a specific session key from a proxy.
	DeleteKey(proxyHost, username string, opts ...KeyOption) error

	// DeleteKeyOption deletes only secrets specified by the provided key
	// options keeping user's SSH/TLS certificates and private key intact.
	DeleteKeyOption(proxyHost, username string, opts ...KeyOption) error

	// DeleteKeys removes all session keys from disk.
	DeleteKeys() error

	// AddKnownHostKeys adds the public key to the list of known hosts for
	// a hostname.
	AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error

	// GetKnownHostKeys returns all public keys for a hostname.
	GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error)

	// SaveCerts saves trusted TLS certificates of certificate authorities.
	SaveCerts(proxy string, cas []auth.TrustedCerts) error

	// GetCerts gets trusted TLS certificates of certificate authorities.
	GetCerts(proxy string) (*x509.CertPool, error)

	// GetCertsPEM gets trusted TLS certificates of certificate authorities.
	// Each returned byte slice contains an individual PEM block.
	GetCertsPEM(proxy string) ([][]byte, error)
}

// FSLocalKeyStore implements LocalKeyStore interface using the filesystem.
// Here's the file layout for the FS store:
//
// ~/.tsh/
// ├── known_hosts                   --> trusted certificate authorities (their keys) in a format similar to known_hosts
// └── keys
//    ├── one.example.com            --> Proxy hostname
//    │   ├── certs.pem              --> TLS CA certs for the Teleport CA
//    │   ├── foo                    --> RSA Private Key for user "foo"
//    │   ├── foo-cert.pub           --> SSH certificate for proxies and nodes
//    │   ├── foo.pub                --> Public Key
//    │   ├── foo-x509.pem           --> TLS client certificate for Auth Server
//    │   ├── foo-kube               --> Kubernetes certs for user "foo"
//    │   |   ├── root               --> Kubernetes certs for teleport cluster "root"
//    │   |   │   ├── kubeA-x509.pem --> TLS cert for Kubernetes cluster "kubeA"
//    │   |   │   └── kubeB-x509.pem --> TLS cert for Kubernetes cluster "kubeB"
//    │   |   └── leaf               --> Kubernetes certs for teleport cluster "leaf"
//    │   |       └── kubeC-x509.pem --> TLS cert for Kubernetes cluster "kubeC"
//    |   └── foo-db                 --> Database access certs for user "foo"
//    |       ├── root               --> Database access certs for cluster "root"
//    │       │   ├── dbA-x509.pem   --> TLS cert for database service "dbA"
//    │       │   └── dbB-x509.pem   --> TLS cert for database service "dbB"
//    │       └── leaf               --> Database access certs for cluster "leaf"
//    │           └── dbC-x509.pem   --> TLS cert for database service "dbC"
//    └── two.example.com
//        ├── certs.pem
//        ├── bar
//        ├── bar-cert.pub
//        ├── bar.pub
//        └── bar-x509.pem
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
			log: logrus.WithFields(logrus.Fields{
				trace.Component: teleport.ComponentKeyStore,
			}),
			KeyDir: dirPath,
		},
	}, nil
}

// AddKey adds a new key to the session store. If a key for the host is already
// stored, overwrites it.
func (fs *FSLocalKeyStore) AddKey(host, username string, key *Key) error {
	dirPath := fs.dirFor(host)
	if err := os.MkdirAll(dirPath, profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}
	writeBytes := func(fname string, data []byte) error {
		fp := filepath.Join(dirPath, fname)
		err := ioutil.WriteFile(fp, data, keyFilePerms)
		if err != nil {
			fs.log.Error(err)
		}
		return err
	}
	if err := writeBytes(username+fileExtCert, key.Cert); err != nil {
		return trace.Wrap(err)
	}
	if err := writeBytes(username+fileExtTLSCert, key.TLSCert); err != nil {
		return trace.Wrap(err)
	}
	if err := writeBytes(username+fileExtPub, key.Pub); err != nil {
		return trace.Wrap(err)
	}
	if err := writeBytes(username, key.Priv); err != nil {
		return trace.Wrap(err)
	}
	// TODO(awly): unit test this.
	kubeDir := filepath.Join(dirPath, username+kubeDirSuffix, key.ClusterName)
	// Clean up any old kube certs.
	if err := os.RemoveAll(kubeDir); err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(kubeDir, os.ModeDir|profileDirPerms); err != nil {
		return trace.Wrap(err)
	}
	for kubeCluster, cert := range key.KubeTLSCerts {
		// Prevent directory traversal via a crafted kubernetes cluster name.
		//
		// This will confuse cluster cert loading (GetKey will return
		// kubernetes cluster names different from the ones stored here), but I
		// don't expect any well-meaning user to create bad names.
		kubeCluster = filepath.Clean(kubeCluster)

		fname := filepath.Join(username+kubeDirSuffix, key.ClusterName, kubeCluster+fileExtTLSCert)
		if err := writeBytes(fname, cert); err != nil {
			return trace.Wrap(err)
		}
	}
	for db, cert := range key.DBTLSCerts {
		fname := filepath.Join(username+dbDirSuffix, key.ClusterName, filepath.Clean(db)+fileExtTLSCert)
		if err := os.MkdirAll(filepath.Join(dirPath, filepath.Dir(fname)), os.ModeDir|profileDirPerms); err != nil {
			return trace.Wrap(err)
		}
		if err := writeBytes(fname, cert); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteKey deletes a key from the local store
func (fs *FSLocalKeyStore) DeleteKey(host, username string, opts ...KeyOption) error {
	dirPath := fs.dirFor(host)
	files := []string{
		filepath.Join(dirPath, username+fileExtCert),
		filepath.Join(dirPath, username+fileExtTLSCert),
		filepath.Join(dirPath, username+fileExtPub),
		filepath.Join(dirPath, username),
	}
	for _, fn := range files {
		if err := os.Remove(fn); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, o := range opts {
		if err := o.deleteKey(fs, keyIndex{proxyHost: host, username: username}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteKeyOption deletes only secrets specified by the provided key options
// keeping user's SSH/TLS certificates and private key intact.
//
// Useful when needing to log out of a specific service, like a particular
// database proxy.
func (fs *FSLocalKeyStore) DeleteKeyOption(host, username string, opts ...KeyOption) error {
	for _, o := range opts {
		if err := o.deleteKey(fs, keyIndex{proxyHost: host, username: username}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteKeys removes all session keys from disk.
func (fs *FSLocalKeyStore) DeleteKeys() error {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir)

	err := os.RemoveAll(dirPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetKey returns a key for a given host. If the key is not found,
// returns trace.NotFound error.
func (fs *FSLocalKeyStore) GetKey(proxyHost, username string, opts ...KeyOption) (*Key, error) {
	dirPath := fs.dirFor(proxyHost)
	_, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, trace.NotFound("no session keys for %v in %v", username, proxyHost)
	}

	certFile := filepath.Join(dirPath, username+fileExtCert)
	cert, err := ioutil.ReadFile(certFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	tlsCertFile := filepath.Join(dirPath, username+fileExtTLSCert)
	tlsCert, err := ioutil.ReadFile(tlsCertFile)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	pub, err := ioutil.ReadFile(filepath.Join(dirPath, username+fileExtPub))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	priv, err := ioutil.ReadFile(filepath.Join(dirPath, username))
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}
	tlsCA, err := fs.GetCertsPEM(proxyHost)
	if err != nil {
		fs.log.Error(err)
		return nil, trace.Wrap(err)
	}

	key := &Key{
		Pub:       pub,
		Priv:      priv,
		Cert:      cert,
		ProxyHost: proxyHost,
		TLSCert:   tlsCert,
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: tlsCA,
		}},
		KubeTLSCerts: make(map[string][]byte),
		DBTLSCerts:   make(map[string][]byte),
	}

	for _, o := range opts {
		if err := o.getKey(fs, keyIndex{proxyHost: proxyHost, username: username}, key); err != nil {
			fs.log.Error(err)
			return nil, trace.Wrap(err)
		}
	}

	// Validate the key loaded from disk.
	err = key.CheckCert()
	if err != nil {
		// KeyStore should return expired certificates as well
		if !utils.IsCertExpiredError(err) {
			return nil, trace.Wrap(err)
		}
	}
	sshCertExpiration, err := key.CertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCertExpiration, err := key.TeleportTLSCertValidBefore()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Note, we may be returning expired certificates here, that is okay. If the
	// certificates is expired, it's the responsibility of the TeleportClient to
	// perform cleanup of the certificates and the profile.
	fs.log.Debugf("Returning SSH certificate %q valid until %q, TLS certificate %q valid until %q.",
		certFile, sshCertExpiration, tlsCertFile, tlsCertExpiration)

	return key, nil
}

type keyIndex struct {
	proxyHost   string
	username    string
	clusterName string
}

// KeyOption is an additional step to run when loading (LocalKeyStore.GetKey)
// or deleting (LocalKeyStore.DeleteKey) keys. These are the steps skipped by
// default to reduce the amount of work that Get/DeleteKey performs by default.
type KeyOption interface {
	getKey(store LocalKeyStore, idx keyIndex, key *Key) error
	deleteKey(store LocalKeyStore, idx keyIndex) error
}

// WithKubeCerts returns a GetKeyOption to load kubernetes certificates from
// the store for a given teleport cluster.
func WithKubeCerts(teleportClusterName string) KeyOption {
	return withKubeCerts{teleportClusterName: teleportClusterName}
}

type withKubeCerts struct {
	teleportClusterName string
}

// TODO(awly): unit test this.
func (o withKubeCerts) getKey(store LocalKeyStore, idx keyIndex, key *Key) error {
	switch s := store.(type) {
	case *FSLocalKeyStore:
		dirPath := s.dirFor(idx.proxyHost)
		kubeDir := filepath.Join(dirPath, idx.username+kubeDirSuffix, o.teleportClusterName)
		kubeFiles, err := ioutil.ReadDir(kubeDir)
		if err != nil && !os.IsNotExist(err) {
			return trace.Wrap(err)
		}
		if key.KubeTLSCerts == nil {
			key.KubeTLSCerts = make(map[string][]byte)
		}
		for _, fi := range kubeFiles {
			data, err := ioutil.ReadFile(filepath.Join(kubeDir, fi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
			kubeCluster := strings.TrimSuffix(filepath.Base(fi.Name()), fileExtTLSCert)
			key.KubeTLSCerts[kubeCluster] = data
		}
	case *MemLocalKeyStore:
		idx.clusterName = o.teleportClusterName
		stored, ok := s.inMem[idx]
		if !ok {
			return trace.NotFound("key for %v not found", idx)
		}
		key.KubeTLSCerts = stored.KubeTLSCerts
	default:
		return trace.BadParameter("unexpected key store type %T", store)
	}
	if key.ClusterName == "" {
		key.ClusterName = o.teleportClusterName
	}
	return nil
}

func (o withKubeCerts) deleteKey(store LocalKeyStore, idx keyIndex) error {
	switch s := store.(type) {
	case *FSLocalKeyStore:
		dirPath := s.dirFor(idx.proxyHost)
		kubeCertsDir := filepath.Join(dirPath, idx.username+kubeDirSuffix, o.teleportClusterName)
		if err := os.RemoveAll(kubeCertsDir); err != nil {
			return trace.Wrap(err)
		}
	case *MemLocalKeyStore:
		idx.clusterName = o.teleportClusterName
		stored, ok := s.inMem[idx]
		if ok {
			stored.KubeTLSCerts = nil
		}
	default:
		return trace.BadParameter("unexpected key store type %T", store)
	}
	return nil
}

// WithDBCerts returns a GetKeyOption to load database access certificates
// from the store for a given Teleport cluster.
func WithDBCerts(teleportClusterName, dbName string) KeyOption {
	return withDBCerts{teleportClusterName: teleportClusterName, dbName: dbName}
}

type withDBCerts struct {
	teleportClusterName, dbName string
}

func (o withDBCerts) getKey(store LocalKeyStore, idx keyIndex, key *Key) error {
	switch s := store.(type) {
	case *FSLocalKeyStore:
		dirPath := s.dirFor(idx.proxyHost)
		dbDir := filepath.Join(dirPath, idx.username+dbDirSuffix, o.teleportClusterName)
		dbFiles, err := ioutil.ReadDir(dbDir)
		if err != nil && !os.IsNotExist(err) {
			return trace.Wrap(err)
		}
		if key.DBTLSCerts == nil {
			key.DBTLSCerts = make(map[string][]byte)
		}
		for _, fi := range dbFiles {
			data, err := ioutil.ReadFile(filepath.Join(dbDir, fi.Name()))
			if err != nil {
				return trace.Wrap(err)
			}
			dbName := strings.TrimSuffix(filepath.Base(fi.Name()), fileExtTLSCert)
			key.DBTLSCerts[dbName] = data
		}
	case *MemLocalKeyStore:
		idx.clusterName = o.teleportClusterName
		stored, ok := s.inMem[idx]
		if !ok {
			return trace.NotFound("key for %v not found", idx)
		}
		key.DBTLSCerts = stored.DBTLSCerts
	default:
		return trace.BadParameter("unexpected key store type %T", store)
	}
	if key.ClusterName == "" {
		key.ClusterName = o.teleportClusterName
	}
	return nil
}

func (o withDBCerts) deleteKey(store LocalKeyStore, idx keyIndex) error {
	// If database name is specified, remove only that cert, otherwise remove
	// certs for all databases a user is logged into.
	switch s := store.(type) {
	case *FSLocalKeyStore:
		dirPath := s.dirFor(idx.proxyHost)
		if o.dbName != "" {
			return os.Remove(filepath.Join(dirPath, idx.username+dbDirSuffix, o.teleportClusterName, o.dbName+fileExtTLSCert))
		}
		return os.RemoveAll(filepath.Join(dirPath, idx.username+dbDirSuffix, o.teleportClusterName))
	case *MemLocalKeyStore:
		idx.clusterName = o.teleportClusterName
		stored, ok := s.inMem[idx]
		if !ok {
			return trace.NotFound("key for %v not found", idx)
		}
		if o.dbName != "" {
			stored.DBTLSCerts[o.dbName] = nil
		} else {
			stored.DBTLSCerts = nil
		}
	default:
		return trace.BadParameter("unexpected key store type %T", store)
	}
	return nil
}

// initKeysDir initializes the keystore root directory. Usually it is ~/.tsh
func initKeysDir(dirPath string) (string, error) {
	var err error
	// not specified? use `~/.tsh`
	if dirPath == "" {
		u, err := user.Current()
		if err != nil {
			dirPath = os.TempDir()
		} else {
			dirPath = u.HomeDir
		}
		dirPath = filepath.Join(dirPath, defaultKeyDir)
	}
	// create if doesn't exist:
	_, err = os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(dirPath, os.ModeDir|profileDirPerms)
			if err != nil {
				return "", trace.ConvertSystemError(err)
			}
		} else {
			return "", trace.Wrap(err)
		}
	}

	return dirPath, nil
}

type fsLocalNonSessionKeyStore struct {
	// log holds the structured logger.
	log *logrus.Entry

	// KeyDir is the directory where all keys are stored.
	KeyDir string
}

// dirFor returns the path to the session keys for a given host. The value
// for fs.KeyDir is typically "~/.tsh", sessionKeyDir is typically "keys",
// and proxyHost typically has values like "proxy.example.com".
func (fs *fsLocalNonSessionKeyStore) dirFor(proxyHost string) string {
	return filepath.Join(fs.KeyDir, sessionKeyDir, proxyHost)
}

// GetCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (fs *fsLocalNonSessionKeyStore) GetCertsPEM(proxy string) ([][]byte, error) {
	dir := fs.dirFor(proxy)
	data, err := ioutil.ReadFile(filepath.Join(dir, fileNameTLSCerts))
	if err != nil {
		return nil, trace.Wrap(err)
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

// GetCerts returns trusted TLS certificates of certificate authorities as
// x509.CertPool.
func (fs *fsLocalNonSessionKeyStore) GetCerts(proxy string) (*x509.CertPool, error) {
	blocks, err := fs.GetCertsPEM(proxy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, bytes := range blocks {
		block, _ := pem.Decode(bytes)
		if block == nil {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, trace.BadParameter("failed to parse certificate: %v", err)
		}
		fs.log.Debugf("Adding trusted cluster certificate authority %q to trusted pool.", cert.Issuer)
		pool.AddCert(cert)
	}
	return pool, nil
}

// GetKnownHostKeys returns all known public keys from 'known_hosts'
func (fs *fsLocalNonSessionKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(fs.KeyDir, fileNameKnownHosts))
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

// AddKnownHostKeys adds a new entry to 'known_hosts' file
func (fs *fsLocalNonSessionKeyStore) AddKnownHostKeys(hostname string, hostKeys []ssh.PublicKey) (retErr error) {
	fp, err := os.OpenFile(filepath.Join(fs.KeyDir, fileNameKnownHosts), os.O_CREATE|os.O_RDWR, 0640)
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

// SaveCerts saves trusted TLS certificates of certificate authorities
func (fs *fsLocalNonSessionKeyStore) SaveCerts(proxy string, cas []auth.TrustedCerts) (retErr error) {
	dir := fs.dirFor(proxy)
	if err := os.MkdirAll(dir, profileDirPerms); err != nil {
		fs.log.Error(err)
		return trace.ConvertSystemError(err)
	}

	fp, err := os.OpenFile(filepath.Join(dir, fileNameTLSCerts), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
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

// noLocalKeyStore is a LocalKeyStore representing the absence of a keystore.
// All methods return errors. This exists to avoid nil checking everywhere in
// LocalKeyAgent and prevent nil pointer panics.
type noLocalKeyStore struct{}

var errNoLocalKeyStore = trace.NotFound("there is no local keystore")

func (noLocalKeyStore) AddKey(proxy string, username string, key *Key) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetKey(proxy, username string, opts ...KeyOption) (*Key, error) {
	return nil, errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteKey(proxyHost, username string, opts ...KeyOption) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteKeyOption(proxyHost, username string, opts ...KeyOption) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) DeleteKeys() error { return errNoLocalKeyStore }
func (noLocalKeyStore) AddKnownHostKeys(hostname string, keys []ssh.PublicKey) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
	return nil, errNoLocalKeyStore
}
func (noLocalKeyStore) SaveCerts(proxy string, cas []auth.TrustedCerts) error {
	return errNoLocalKeyStore
}
func (noLocalKeyStore) GetCerts(proxy string) (*x509.CertPool, error) { return nil, errNoLocalKeyStore }
func (noLocalKeyStore) GetCertsPEM(proxy string) ([][]byte, error)    { return nil, errNoLocalKeyStore }

// MemLocalKeyStore is an in-memory session keystore implementation.
type MemLocalKeyStore struct {
	fsLocalNonSessionKeyStore
	inMem map[keyIndex]*Key
}

// NewMemLocalKeyStore initializes a MemLocalKeyStore, the key directory here is only used
// for storing CA certificates and known host fingerprints.
func NewMemLocalKeyStore(dirPath string) (*MemLocalKeyStore, error) {
	dirPath, err := initKeysDir(dirPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inMem := make(map[keyIndex]*Key)
	return &MemLocalKeyStore{fsLocalNonSessionKeyStore: fsLocalNonSessionKeyStore{
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyStore,
		}),
		KeyDir: dirPath,
	}, inMem: inMem}, nil
}

// AddKey writes a key to the underlying key store.
func (s *MemLocalKeyStore) AddKey(proxy string, username string, key *Key) error {
	s.inMem[keyIndex{proxyHost: proxy, username: username}] = key
	if key.ClusterName != "" {
		s.inMem[keyIndex{proxyHost: proxy, username: username, clusterName: key.ClusterName}] = key
	}
	return nil
}

// GetKey returns the session key for the given username and proxy.
func (s *MemLocalKeyStore) GetKey(proxy, username string, opts ...KeyOption) (*Key, error) {
	idx := keyIndex{proxyHost: proxy, username: username}
	entry, ok := s.inMem[idx]
	if !ok {
		return nil, trace.NotFound("key for %v not found", idx)
	}
	for _, o := range opts {
		if err := o.getKey(s, idx, entry); err != nil {
			s.log.Error(err)
			return nil, trace.Wrap(err)
		}
	}
	return entry, nil
}

// DeleteKey removes a specific session key from a proxy.
func (s *MemLocalKeyStore) DeleteKey(proxyHost, username string, opts ...KeyOption) error {
	delete(s.inMem, keyIndex{proxyHost: proxyHost, username: username})
	s.DeleteKeyOption(proxyHost, username, opts...)
	return nil
}

// DeleteKeys removes all session keys.
func (s *MemLocalKeyStore) DeleteKeys() error {
	s.inMem = make(map[keyIndex]*Key)
	return nil
}

// DeleteKeyOption deletes only secrets specified by the provided key
// options keeping user's SSH/TLS certificates and private key intact.
func (s *MemLocalKeyStore) DeleteKeyOption(proxyHost, username string, opts ...KeyOption) error {
	for _, o := range opts {
		if err := o.deleteKey(s, keyIndex{proxyHost: proxyHost, username: username}); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
