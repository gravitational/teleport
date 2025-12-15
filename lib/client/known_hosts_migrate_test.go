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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/sshca"
)

type knownHostsMigrateTest struct {
	keygen *testauthority.Keygen
}

func newMigrateTest() knownHostsMigrateTest {
	return knownHostsMigrateTest{
		keygen: testauthority.New(),
	}
}

func generateHostCert(t *testing.T, s *knownHostsMigrateTest, clusterName string) []byte {
	_, hostPub, err := s.keygen.GenerateKeyPair()
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(CAPriv)
	require.NoError(t, err)

	cert, err := s.keygen.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		HostID:        "127.0.0.1",
		NodeName:      "127.0.0.1",
		PublicHostKey: hostPub,
		Identity: sshca.Identity{
			ClusterName: clusterName,
		},
	})
	require.NoError(t, err)

	return cert
}

func generateOldHostEntry(
	t *testing.T, s *knownHostsMigrateTest, cert []byte, clusterName string,
) *knownHostEntry {
	formatted := fmt.Sprintf("%s %s", clusterName, strings.TrimSpace(string(cert)))
	entry, err := parseKnownHost(formatted)
	require.NoError(t, err)
	require.Equal(t, formatted, entry.raw)

	return entry
}

func generateNewHostEntry(
	t *testing.T, s *knownHostsMigrateTest, cert []byte, clusterName string, proxyName string,
) *knownHostEntry {
	formatted := fmt.Sprintf(
		"@cert-authority %s,%s,*.%s %s type=host",
		proxyName, clusterName, clusterName, strings.TrimSpace(string(cert)),
	)
	entry, err := parseKnownHost(formatted)
	require.NoError(t, err)
	require.Equal(t, formatted, entry.raw)

	return entry
}

func TestParseKnownHost(t *testing.T) {
	s := newMigrateTest()

	oldCert := generateHostCert(t, &s, "example.com")
	oldEntry := generateOldHostEntry(t, &s, oldCert, "example.com")

	require.Empty(t, oldEntry.comment)
	require.Empty(t, oldEntry.marker)
	require.Equal(t, []string{"example.com"}, oldEntry.hosts)

	oldCertParsed, _, _, _, err := ssh.ParseAuthorizedKey(oldCert)
	require.NoError(t, err)
	require.True(t, bytes.Equal(oldCertParsed.Marshal(), oldEntry.pubKey.Marshal()))

	newCert := generateHostCert(t, &s, "example.com")
	newEntry := generateNewHostEntry(t, &s, newCert, "example.com", "proxy.example.com")

	require.Equal(t, "cert-authority", newEntry.marker)
	require.Equal(t, []string{"proxy.example.com", "example.com", "*.example.com"}, newEntry.hosts)
	require.Equal(t, "type=host", newEntry.comment)

	newCertParsed, _, _, _, err := ssh.ParseAuthorizedKey(newCert)
	require.NoError(t, err)
	require.True(t, bytes.Equal(newCertParsed.Marshal(), newEntry.pubKey.Marshal()))
}

func TestIsOldHostsEntry(t *testing.T) {
	s := newMigrateTest()

	// tsh's older format.
	cert := generateHostCert(t, &s, "example.com")
	oldEntry := generateOldHostEntry(t, &s, cert, "example.com")
	require.True(t, isOldStyleHostsEntry(oldEntry))

	// tsh's new format.
	newEntry := generateNewHostEntry(t, &s, cert, "example.com", "proxy.example.com")
	require.False(t, isOldStyleHostsEntry(newEntry))

	// Also test an invalid old cert to ensure it won't be accidentally pruned.
	// In this case, multiple hosts should invalidate the key.
	hostsEntryString := fmt.Sprintf("foo,bar %s", strings.TrimSpace(string(cert)))
	hostsEntry, err := parseKnownHost(hostsEntryString)
	require.NoError(t, err)
	require.False(t, isOldStyleHostsEntry(hostsEntry))

	// Additionally, any comment invalidates it.
	commentEntryString := fmt.Sprintf("foo %s comment", strings.TrimSpace(string(cert)))
	commentEntry, err := parseKnownHost(commentEntryString)
	require.NoError(t, err)
	require.False(t, isOldStyleHostsEntry(commentEntry))
}

func TestCanPruneOldHostsEntry(t *testing.T) {
	s := newMigrateTest()

	certFoo := generateHostCert(t, &s, "foo.example.com")
	certLeaf := generateHostCert(t, &s, "leaf.example.com")
	certBar := generateHostCert(t, &s, "bar.example.com")
	oldEntry := generateOldHostEntry(t, &s, certFoo, "foo.example.com")

	// Valid new entries.
	newValidFoo := generateNewHostEntry(t, &s, certFoo, "foo.example.com", "proxy.foo.example.com")
	newValidLeaf := generateNewHostEntry(t, &s, certLeaf, "leaf.example.com", "proxy.foo.example.com")

	// An entry with a non-matching certificate for its hostname.
	newInvalidFoo := generateNewHostEntry(t, &s, certBar, "foo.example.com", "proxy.foo.example.com")

	// An entry with a non-matching hostname for its certificate.
	newInvalidBar := generateNewHostEntry(t, &s, certFoo, "bar.example.com", "proxy.bar.example.com")

	// Do not prune an old entry if no new entries exist.
	require.False(t, canPruneOldHostsEntry(oldEntry, []*knownHostEntry{}))

	// Do not prune an old entry if the certificate and hostname don't match.
	require.False(t, canPruneOldHostsEntry(oldEntry, []*knownHostEntry{newInvalidFoo}))
	require.False(t, canPruneOldHostsEntry(oldEntry, []*knownHostEntry{newInvalidBar}))

	// Prune an entry even if it's not the first in the list.
	require.True(t, canPruneOldHostsEntry(oldEntry, []*knownHostEntry{newValidLeaf, newValidFoo}))
}

func TestPruneOldHostKeys(t *testing.T) {
	s := newMigrateTest()

	certFoo := generateHostCert(t, &s, "foo.example.com")
	certLeaf := generateHostCert(t, &s, "leaf.example.com")
	certBar := generateHostCert(t, &s, "bar.example.com")
	certBaz := generateHostCert(t, &s, "baz.example.com")

	allOldEntries := []string{
		generateOldHostEntry(t, &s, certFoo, "foo.example.com").raw,
		generateOldHostEntry(t, &s, certLeaf, "leaf.example.com").raw,
		generateOldHostEntry(t, &s, certBar, "bar.example.com").raw,
	}
	allNewEntries := []string{
		generateNewHostEntry(t, &s, certFoo, "foo.example.com", "proxy.foo.example.com").raw,
		generateNewHostEntry(t, &s, certLeaf, "leaf.example.com", "proxy.foo.example.com").raw,
		generateNewHostEntry(t, &s, certBar, "bar.example.com", "proxy.bar.example.com").raw,
		generateNewHostEntry(t, &s, certBaz, "baz.example.com", "proxy.baz.example.com").raw,
	}
	allEntries := append(allOldEntries, allNewEntries...)

	// If only old or only new entries, prune nothing.
	require.ElementsMatch(t, pruneOldHostKeys(allOldEntries), allOldEntries)
	require.ElementsMatch(t, pruneOldHostKeys(allNewEntries), allNewEntries)

	// If only unmatched entries, prune nothing. Sort order may change.
	unmatchedEntries := append(allOldEntries, allNewEntries[3]) // Append baz.
	require.ElementsMatch(t, pruneOldHostKeys(unmatchedEntries), unmatchedEntries)

	// Only prune one entry (bar.example.com).
	require.ElementsMatch(
		t,
		pruneOldHostKeys(append(allOldEntries, allNewEntries[2])),
		append(allOldEntries[0:2], allNewEntries[2]),
	)

	// Only prune a subset (leaf cluster scenario: foo.example.com, leaf.example.com).
	require.ElementsMatch(
		t,
		pruneOldHostKeys(append(allOldEntries, allNewEntries[0], allNewEntries[1])),
		append(allNewEntries[0:2], allOldEntries[2]),
	)

	// Prune everything at once - unlikely in practice, but should still succeed.
	require.ElementsMatch(t, pruneOldHostKeys(allEntries), allNewEntries)
}
