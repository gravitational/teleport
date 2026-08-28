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
	"bufio"
	"bytes"
	"context"
	"encoding/pem"
	"fmt"
	iofs "io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// TrustedCertsStore is a storage interface for trusted CA certificates and public keys.
type TrustedCertsStore interface {
	// SaveTrustedCerts adds the given trusted CA TLS certificates and SSH host keys to the store.
	// Existing TLS certificates for the given trusted certs will be overwritten, while host keys
	// will be appended to existing entries.
	SaveTrustedCerts(proxyHost string, cas []authclient.TrustedCerts) error

	// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
	GetTrustedCerts(proxyHost string) ([]authclient.TrustedCerts, error)

	// GetTrustedCertsPEM gets trusted TLS certificates of certificate authorities.
	// Each returned byte slice contains an individual PEM block.
	GetTrustedCertsPEM(proxyHost string) ([][]byte, error)

	// GetTrustedHostKeys returns all trusted public host keys. If hostnames are provided, only
	// matching host keys will be returned. Host names should be a proxy host or cluster name.
	GetTrustedHostKeys(hostnames ...string) ([]ssh.PublicKey, error)
}

// MemTrustedCertsStore is an in-memory implementation of TrustedCertsStore.
type MemTrustedCertsStore struct {
	// memLocalCAStoreMap is a two-dimensinoal map indexed by [proxyHost][clusterName]
	trustedCerts trustedCertsMap
}

// trustedCertsMap is a two-dimensinoal map indexed by [proxyHost][clusterName]
type trustedCertsMap map[string]map[string]authclient.TrustedCerts

// NewMemTrustedCertsStore creates a new instance of MemTrustedCertsStore.
func NewMemTrustedCertsStore() *MemTrustedCertsStore {
	return &MemTrustedCertsStore{
		trustedCerts: make(trustedCertsMap),
	}
}

// SaveTrustedCerts saves trusted TLS certificates of certificate authorities.
func (ms *MemTrustedCertsStore) SaveTrustedCerts(proxyHost string, cas []authclient.TrustedCerts) error {
	if proxyHost == "" {
		return trace.BadParameter("proxyHost must be provided to add trusted certs")
	}
	_, ok := ms.trustedCerts[proxyHost]
	if !ok {
		ms.trustedCerts[proxyHost] = map[string]authclient.TrustedCerts{}
	}
	for _, ca := range cas {
		if ca.ClusterName == "" {
			return trace.BadParameter("trusted certs entry cannot have an empty cluster name")
		}

		entry, ok := ms.trustedCerts[proxyHost][ca.ClusterName]
		if !ok {
			entry = authclient.TrustedCerts{ClusterName: ca.ClusterName}
		}

		// If TLS certificates were provided, replace the existing entry's certs.
		if len(ca.TLSCertificates) != 0 {
			entry.TLSCertificates = ca.TLSCertificates
		}

		// Unlike with trusted TLS certificates, we don't replace the trusted host keys.
		// Instead, append to the existing entry, without duplicates. This matches the
		// behavior of the known hosts file.
		entry.AuthorizedKeys = apiutils.DeduplicateAny(append(entry.AuthorizedKeys, ca.AuthorizedKeys...), bytes.Equal)

		ms.trustedCerts[proxyHost][ca.ClusterName] = entry
	}

	return nil
}

// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
func (ms *MemTrustedCertsStore) GetTrustedCerts(proxyHost string) ([]authclient.TrustedCerts, error) {
	var trustedCerts []authclient.TrustedCerts
	for _, entry := range ms.trustedCerts[proxyHost] {
		trustedCerts = append(trustedCerts, entry)
	}
	return trustedCerts, nil
}

// GetTrustedCertsPEM gets trusted TLS certificates of certificate authorities.
// Each returned byte slice contains an individual PEM block.
func (ms *MemTrustedCertsStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	var tlsHostCerts [][]byte
	for _, ca := range ms.trustedCerts[proxyHost] {
		tlsHostCerts = append(tlsHostCerts, ca.TLSCertificates...)
	}
	return tlsHostCerts, nil
}

// GetTrustedHostKeys returns all trusted public host keys. If hostnames are provided, only
// matching host keys will be returned. Host names should be a proxy host or cluster name.
func (ms *MemTrustedCertsStore) GetTrustedHostKeys(hostnames ...string) ([]ssh.PublicKey, error) {
	// authorized hosts are not retrieved by proxyHost, only clusterName, so we search all proxy entries.
	var hostKeys []ssh.PublicKey
	for proxyHost, proxyEntries := range ms.trustedCerts {
		for _, entry := range proxyEntries {
			// Mirror the hosts we would find in a known_hosts entry.
			hosts := []string{proxyHost, entry.ClusterName, "*." + entry.ClusterName}
			if len(hostnames) == 0 || apisshutils.HostNameMatch(hostnames, hosts) {
				clusterHostKeys, err := apisshutils.ParseAuthorizedKeys(entry.AuthorizedKeys)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				hostKeys = append(hostKeys, clusterHostKeys...)
			}
		}
	}

	return hostKeys, nil
}

// FSTrustedCertsStore is an on-disk implementation of the TrustedCAStore interface.
//
// The FS store uses the file layout outlined in `api/utils/keypaths.go`.
type FSTrustedCertsStore struct {
	// log holds the structured logger.
	log *slog.Logger

	// Dir is the directory where all keys are stored.
	Dir string
}

// NewFSTrustedCertsStore creates a new instance of FSTrustedCertsStore.
func NewFSTrustedCertsStore(dirPath string) *FSTrustedCertsStore {
	dirPath = profile.FullProfilePath(dirPath)
	return &FSTrustedCertsStore{
		log: slog.With(teleport.ComponentKey, teleport.ComponentKeyStore),
		Dir: dirPath,
	}
}

// knownHostsPath returns the known_hosts file path.
func (fs *FSTrustedCertsStore) knownHostsPath() string {
	return keypaths.KnownHostsPath(fs.Dir)
}

// proxyKeyDir returns the keys directory for the given proxy.
func (fs *FSTrustedCertsStore) proxyKeyDir(proxy string) string {
	return keypaths.ProxyKeyDir(fs.Dir, proxy)
}

// casDir returns path to trusted clusters certificates directory.
func (fs *FSTrustedCertsStore) casDir(proxy string) string {
	return keypaths.CAsDir(fs.Dir, proxy)
}

// clusterCAPath returns path to trusted cluster certificate.
func (fs *FSTrustedCertsStore) clusterCAPath(proxy, clusterName string) string {
	return keypaths.TLSCAsPathCluster(fs.Dir, proxy, clusterName)
}

// tlsCAsPath returns the TLS CA certificates legacy path for the given KeyRingIndex.
func (fs *FSTrustedCertsStore) tlsCAsPath(proxy string) string {
	return keypaths.TLSCAsPath(fs.Dir, proxy)
}

// GetTrustedCerts gets the trusted CA TLS certificates and SSH host keys for the given proxyHost.
func (fs *FSTrustedCertsStore) GetTrustedCerts(proxyHost string) ([]authclient.TrustedCerts, error) {
	tlsCA, err := fs.GetTrustedCertsPEM(proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	knownHostsFile, err := fs.getKnownHostsFile()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	knownHosts, err := sshutils.UnmarshalKnownHosts([][]byte{knownHostsFile})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var knownHostsForProxy []sshutils.KnownHost
	for _, kh := range knownHosts {
		if kh.ProxyHost == proxyHost {
			knownHostsForProxy = append(knownHostsForProxy, kh)
		}
	}
	return TrustedCertsFromCACerts(tlsCA, knownHostsForProxy)
}

// GetTrustedHostKeys returns all trusted public host keys. If hostnames are provided, only
// matching host keys will be returned. Host names should be a proxy host or cluster name.
func (fs *FSTrustedCertsStore) GetTrustedHostKeys(hostnames ...string) (keys []ssh.PublicKey, retErr error) {
	knownHosts, err := fs.getKnownHostsFile()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Return all known host keys with one of the given cluster names or proxyHost as a hostname.
	return apisshutils.ParseKnownHosts([][]byte{knownHosts}, hostnames...)
}

func (fs *FSTrustedCertsStore) getKnownHostsFile() (knownHosts []byte, retErr error) {
	unlock, err := utils.FSTryReadLockTimeout(context.Background(), fs.knownHostsPath(), 5*time.Second)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
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

// SaveTrustedCerts saves trusted TLS certificates of certificate authorities.
func (fs *FSTrustedCertsStore) SaveTrustedCerts(proxyHost string, cas []authclient.TrustedCerts) (retErr error) {
	if proxyHost == "" {
		return trace.BadParameter("proxyHost must be provided to add trusted certs")
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

	if err := fs.addKnownHosts(proxyHost, cas); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (fs *FSTrustedCertsStore) saveTrustedCertsInCASDir(proxyHost string, cas []authclient.TrustedCerts) error {
	casDirPath := filepath.Join(fs.casDir(proxyHost))
	if err := os.MkdirAll(casDirPath, os.ModeDir|profileDirPerms); err != nil {
		return trace.ConvertSystemError(err)
	}

	for _, ca := range cas {
		if len(ca.TLSCertificates) == 0 {
			continue
		}
		// check if cluster name is safe and doesn't contain miscellaneous characters.
		if strings.Contains(ca.ClusterName, "..") {
			fs.log.WarnContext(context.Background(), "Skipped unsafe cluster name", "cluster_name", ca.ClusterName)
			continue
		}
		// Create CA files in cas dir for each cluster.
		if err := fs.writeClusterCertificates(proxyHost, ca.ClusterName, ca.TLSCertificates); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (fs *FSTrustedCertsStore) writeClusterCertificates(proxyHost, clusterName string, tlsCertificates [][]byte) (retErr error) {
	caFile, err := os.OpenFile(fs.clusterCAPath(proxyHost, clusterName), os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o640)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer caFile.Close()

	for _, cert := range tlsCertificates {
		if _, err := caFile.Write(cert); err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	if err := caFile.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

func (fs *FSTrustedCertsStore) saveTrustedCertsInLegacyCAFile(proxyHost string, cas []authclient.TrustedCerts) (retErr error) {
	if err := os.MkdirAll(fs.proxyKeyDir(proxyHost), os.ModeDir|profileDirPerms); err != nil {
		return trace.ConvertSystemError(err)
	}

	certsFile := fs.tlsCAsPath(proxyHost)
	fp, err := os.OpenFile(certsFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o640)
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
	if err := fp.Sync(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

// addKnownHosts adds new entries to `known_hosts` file for the provided CAs.
func (fs *FSTrustedCertsStore) addKnownHosts(proxyHost string, cas []authclient.TrustedCerts) (retErr error) {
	if err := os.MkdirAll(fs.proxyKeyDir(proxyHost), os.ModeDir|profileDirPerms); err != nil {
		return trace.ConvertSystemError(err)
	}

	// We're trying to serialize our writes to the 'known_hosts' file to avoid corruption, since there
	// are cases when multiple tsh instances will try to write to it.
	unlock, err := utils.FSTryWriteLockTimeout(context.Background(), fs.knownHostsPath(), 5*time.Second)
	if err != nil {
		return trace.WrapWithMessage(err, "could not acquire lock for the `known_hosts` file")
	}
	defer utils.StoreErrorOf(unlock, &retErr)

	fp, err := os.OpenFile(fs.knownHostsPath(), os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer utils.StoreErrorOf(fp.Close, &retErr)

	// read all existing entries into a map (this removes any pre-existing dupes)
	entries := make(map[string]int)
	output := make([]string, 0)
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
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
		for _, hostKey := range ca.AuthorizedKeys {
			fs.log.DebugContext(context.Background(), "Adding known host entry",
				"cluster_name", ca.ClusterName,
				"proxy", proxyHost,
			)

			// Write keys in an OpenSSH-compatible format. A previous format was not
			// quite OpenSSH-compatible, so we may write a duplicate entry here. Any
			// duplicates will be pruned below.
			// We include both the proxy server and original hostname as well as the
			// root domain wildcard. OpenSSH clients match against both the proxy
			// host and nodes (via the wildcard). Teleport itself occasionally uses
			// the root cluster name.
			line, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
				Hostname:      ca.ClusterName,
				ProxyHost:     proxyHost,
				AuthorizedKey: hostKey,
			})
			if err != nil {
				return trace.Wrap(err)
			}

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
		if _, err := fp.Write([]byte(line)); err != nil {
			return trace.Wrap(err)
		}
	}
	return fp.Sync()
}

// GetTrustedCertsPEM returns trusted TLS certificates of certificate authorities PEM
// blocks.
func (fs *FSTrustedCertsStore) GetTrustedCertsPEM(proxyHost string) ([][]byte, error) {
	dir := fs.casDir(proxyHost)

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.ConvertSystemError(err)
	}

	var blocks [][]byte
	err := filepath.Walk(dir, func(path string, info iofs.FileInfo, err error) error {
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
				fs.log.DebugContext(context.Background(), "Skipping PEM block",
					"type", block.Type,
					"headers", block.Headers,
				)
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

// TrustedCertsFromCACerts converts the given TLS CA certificates and KnownHosts files into
// a list of Trusted Certs. If a proxyHost is specified, only known hosts with that proxy host
// as one of its hostnames are returned.
func TrustedCertsFromCACerts(tlsCACerts [][]byte, knownHosts []sshutils.KnownHost) ([]authclient.TrustedCerts, error) {
	clusterCAs := make(map[string]*authclient.TrustedCerts)

	for _, certPEM := range tlsCACerts {
		cert, err := tlsca.ParseCertificatePEM(certPEM)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusterName := cert.Issuer.CommonName
		if entry, ok := clusterCAs[clusterName]; !ok {
			clusterCAs[clusterName] = &authclient.TrustedCerts{
				ClusterName:     clusterName,
				TLSCertificates: [][]byte{certPEM},
			}
		} else {
			entry.TLSCertificates = append(entry.TLSCertificates, certPEM)
		}
	}

	for _, kh := range knownHosts {
		if kh.Hostname == "" {
			continue
		}
		if entry, ok := clusterCAs[kh.Hostname]; !ok {
			clusterCAs[kh.Hostname] = &authclient.TrustedCerts{
				ClusterName:    kh.Hostname,
				AuthorizedKeys: [][]byte{kh.AuthorizedKey},
			}
		} else {
			entry.AuthorizedKeys = append(entry.AuthorizedKeys, kh.AuthorizedKey)
		}
	}

	var trustedCerts []authclient.TrustedCerts
	for _, trustedCA := range clusterCAs {
		trustedCerts = append(trustedCerts, *trustedCA)
	}

	return trustedCerts, nil
}
