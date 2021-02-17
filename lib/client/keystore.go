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
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	defaultKeyDir      = client.ProfileDir
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
	// log holds the structured logger.
	log *logrus.Entry

	// KeyDir is the directory where all keys are stored.
	KeyDir string
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
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentKeyStore,
		}),
		KeyDir: dirPath,
	}, nil
}

// AddKey adds a new key to the session store. If a key for the host is already
// stored, overwrites it.
func (fs *FSLocalKeyStore) AddKey(host, username string, key *Key) error {
	dirPath, err := fs.dirFor(host, true)
	if err != nil {
		return trace.Wrap(err)
	}
	writeBytes := func(fname string, data []byte) error {
		fp := filepath.Join(dirPath, fname)
		err := ioutil.WriteFile(fp, data, keyFilePerms)
		if err != nil {
			fs.log.Error(err)
		}
		return err
	}
	if err = writeBytes(username+fileExtCert, key.Cert); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username+fileExtTLSCert, key.TLSCert); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username+fileExtPub, key.Pub); err != nil {
		return trace.Wrap(err)
	}
	if err = writeBytes(username, key.Priv); err != nil {
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
	dirPath, err := fs.dirFor(host, false)
	if err != nil {
		return trace.Wrap(err)
	}
	files := []string{
		filepath.Join(dirPath, username+fileExtCert),
		filepath.Join(dirPath, username+fileExtTLSCert),
		filepath.Join(dirPath, username+fileExtPub),
		filepath.Join(dirPath, username),
	}
	for _, fn := range files {
		if err = os.Remove(fn); err != nil {
			return trace.Wrap(err)
		}
	}
	for _, o := range opts {
		if err := o.deleteKey(dirPath, username); err != nil {
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
	dirPath, err := fs.dirFor(host, false)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, o := range opts {
		if err := o.deleteKey(dirPath, username); err != nil {
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
	dirPath, err := fs.dirFor(proxyHost, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = ioutil.ReadDir(dirPath)
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
		if err := o.getKey(dirPath, username, key); err != nil {
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

// KeyOption is an additional step to run when loading (LocalKeyStore.GetKey)
// or deleting (LocalKeyStore.DeleteKey) keys. These are the steps skipped by
// default to reduce the amount of work that Get/DeleteKey performs by default.
type KeyOption interface {
	getKey(dirPath, username string, key *Key) error
	deleteKey(dirPath, username string) error
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
func (o withKubeCerts) getKey(dirPath, username string, key *Key) error {
	kubeDir := filepath.Join(dirPath, username+kubeDirSuffix, o.teleportClusterName)
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
	if key.ClusterName == "" {
		key.ClusterName = o.teleportClusterName
	}
	return nil
}

func (o withKubeCerts) deleteKey(dirPath, username string) error {
	kubeCertsDir := filepath.Join(dirPath, username+kubeDirSuffix, o.teleportClusterName)
	if err := os.RemoveAll(kubeCertsDir); err != nil {
		return trace.Wrap(err)
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

func (o withDBCerts) getKey(dirPath, username string, key *Key) error {
	dbDir := filepath.Join(dirPath, username+dbDirSuffix, o.teleportClusterName)
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
	if key.ClusterName == "" {
		key.ClusterName = o.teleportClusterName
	}
	return nil
}

func (o withDBCerts) deleteKey(dirPath, username string) error {
	// If database name is specified, remove only that cert, otherwise remove
	// certs for all databases a user is logged into.
	if o.dbName != "" {
		return os.Remove(filepath.Join(dirPath, username+dbDirSuffix, o.teleportClusterName, o.dbName+fileExtTLSCert))
	}
	return os.RemoveAll(filepath.Join(dirPath, username+dbDirSuffix, o.teleportClusterName))
}

// SaveCerts saves trusted TLS certificates of certificate authorities
func (fs *FSLocalKeyStore) SaveCerts(proxy string, cas []auth.TrustedCerts) error {
	dir, err := fs.dirFor(proxy, true)
	if err != nil {
		return trace.Wrap(err)
	}
	fp, err := os.OpenFile(filepath.Join(dir, fileNameTLSCerts), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fp.Close()
	for _, ca := range cas {
		for _, cert := range ca.TLSCertificates {
			_, err := fp.Write(cert)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			_, err = fp.WriteString("\n")
			if err != nil {
				return trace.ConvertSystemError(err)
			}
		}
	}
	return fp.Sync()
}

// GetCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (fs *FSLocalKeyStore) GetCertsPEM(proxy string) ([][]byte, error) {
	dir, err := fs.dirFor(proxy, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
func (fs *FSLocalKeyStore) GetCerts(proxy string) (*x509.CertPool, error) {
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

// AddKnownHostKeys adds a new entry to 'known_hosts' file
func (fs *FSLocalKeyStore) AddKnownHostKeys(hostname string, hostKeys []ssh.PublicKey) error {
	fp, err := os.OpenFile(filepath.Join(fs.KeyDir, fileNameKnownHosts), os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fp.Close()
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

// GetKnownHostKeys returns all known public keys from 'known_hosts'
func (fs *FSLocalKeyStore) GetKnownHostKeys(hostname string) ([]ssh.PublicKey, error) {
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

// dirFor returns the path to the session keys for a given host. The value
// for fs.KeyDir is typically "~/.tsh", sessionKeyDir is typically "keys",
// and proxyHost typically has values like "proxy.example.com".
//
// If the create flag is true, the directory will be created if it does
// not exist.
func (fs *FSLocalKeyStore) dirFor(proxyHost string, create bool) (string, error) {
	dirPath := filepath.Join(fs.KeyDir, sessionKeyDir, proxyHost)

	if create {
		if err := os.MkdirAll(dirPath, profileDirPerms); err != nil {
			fs.log.Error(err)
			return "", trace.ConvertSystemError(err)
		}
	}

	return dirPath, nil
}

// initKeysDir initializes the keystore root directory. Usually it is ~/.tsh
func initKeysDir(dirPath string) (string, error) {
	var err error
	dirPath = client.FullProfilePath(dirPath)
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
