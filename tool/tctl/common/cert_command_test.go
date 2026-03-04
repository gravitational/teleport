/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/tlsca"
)

// generateTestCert creates a PEM-encoded certificate with the given identity
// and writes it to a temp file, returning the file path.
func generateTestCert(t *testing.T, id tlsca.Identity) string {
	t.Helper()

	clock := clockwork.NewFakeClock()
	expires := clock.Now().Add(1 * time.Hour)
	id.Expires = expires

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	subj, err := id.Subject()
	require.NoError(t, err)

	certPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	certPath := filepath.Join(t.TempDir(), "test.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	return certPath
}

func TestCertInspect_BasicIdentity(t *testing.T) {
	certPath := generateTestCert(t, tlsca.Identity{
		Username:        "alice@example.com",
		Groups:          []string{"access", "editor"},
		Principals:      []string{"root", "ubuntu"},
		TeleportCluster: "example.teleport.sh",
		PinnedIP:        "192.168.1.100",
		LoginIP:         "10.0.0.5",
		BotName:         "my-bot",
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "postgres-main",
			Protocol:    "postgres",
			Username:    "dbadmin",
			Database:    "mydb",
		},
		DeviceExtensions: tlsca.DeviceExtensions{
			DeviceID: "device-123",
			AssetTag: "ASSET-456",
		},
	})

	cmd := &certCommand{certPath: certPath, format: "text"}
	var buf bytes.Buffer
	err := cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	output := buf.String()

	// Verify X509 header.
	assert.Contains(t, output, "X509 Certificate:")
	assert.Contains(t, output, "Serial:")

	// Verify identity fields.
	assert.Contains(t, output, "alice@example.com")
	assert.Contains(t, output, "access, editor")
	assert.Contains(t, output, "root, ubuntu")
	assert.Contains(t, output, "example.teleport.sh")
	assert.Contains(t, output, "192.168.1.100")
	assert.Contains(t, output, "10.0.0.5")
	assert.Contains(t, output, "my-bot")

	// Verify database routing.
	assert.Contains(t, output, "Route To Database:")
	assert.Contains(t, output, "postgres-main")
	assert.Contains(t, output, "postgres")
	assert.Contains(t, output, "dbadmin")

	// Verify device extensions.
	assert.Contains(t, output, "Device Extensions:")
	assert.Contains(t, output, "device-123")
	assert.Contains(t, output, "ASSET-456")

	// Should NOT have scope pin output.
	assert.NotContains(t, output, "Scoped Roles:")
}

func TestCertInspect_ScopePin(t *testing.T) {
	certPath := generateTestCert(t, tlsca.Identity{
		Username:        "bob@example.com",
		TeleportCluster: "scoped.teleport.sh",
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {
					"/staging":      {"staging-reader", "staging-test"},
					"/staging/west": {"staging-west-debug"},
				},
				"/staging": {
					"/staging/west": {"staging-auditor", "staging-editor"},
				},
			}),
		},
	})

	cmd := &certCommand{certPath: certPath, format: "text"}
	var buf bytes.Buffer
	err := cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	output := buf.String()

	// Verify scope pin fields.
	assert.Contains(t, output, "Scope:")
	assert.Contains(t, output, "/staging")
	assert.Contains(t, output, "Scoped Roles:")

	// Verify tree structure characters.
	assert.Contains(t, output, "├──")
	assert.Contains(t, output, "└──")

	// Verify role names appear in the tree.
	assert.Contains(t, output, "staging-reader")
	assert.Contains(t, output, "staging-test")
	assert.Contains(t, output, "staging-west-debug")
	assert.Contains(t, output, "staging-auditor")
	assert.Contains(t, output, "staging-editor")

	// Should NOT have regular (non-scoped) roles line.
	// "Scoped Roles:" is expected, but a standalone "Roles:" line should not appear.
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("Roles:")) {
			t.Errorf("unexpected standalone Roles line: %s", trimmed)
		}
	}
}

func TestCertInspect_JSONOutput(t *testing.T) {
	certPath := generateTestCert(t, tlsca.Identity{
		Username:        "alice@example.com",
		Groups:          []string{"access", "editor"},
		TeleportCluster: "example.teleport.sh",
		PinnedIP:        "192.168.1.100",
	})

	cmd := &certCommand{certPath: certPath, format: "json"}
	var buf bytes.Buffer
	err := cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	var out certInspectOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	assert.Equal(t, "alice@example.com", out.Identity.Username)
	assert.Equal(t, []string{"access", "editor"}, out.Identity.Roles)
	assert.Equal(t, "example.teleport.sh", out.Identity.TeleportCluster)
	assert.Equal(t, "192.168.1.100", out.Identity.PinnedIP)
	assert.NotEmpty(t, out.Certificate.Serial)
}

func TestCertInspect_JSONScopePin(t *testing.T) {
	certPath := generateTestCert(t, tlsca.Identity{
		Username:        "bob@example.com",
		TeleportCluster: "scoped.teleport.sh",
		ScopePin: &scopesv1.Pin{
			Scope: "/staging",
			AssignmentTree: pinning.AssignmentTreeFromMap(map[string]map[string][]string{
				"/": {"/staging": {"reader"}},
			}),
		},
	})

	cmd := &certCommand{certPath: certPath, format: "json"}
	var buf bytes.Buffer
	err := cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	var out certInspectOutput
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))

	assert.Equal(t, "/staging", out.Identity.Scope)
	require.NotNil(t, out.Identity.ScopedRoles)
	assert.Contains(t, out.Identity.ScopedRoles, "/")
	assert.Equal(t, []string{"reader"}, out.Identity.ScopedRoles["/"]["/staging"])
}

func TestCertInspect_InvalidFile(t *testing.T) {
	// Non-existent file.
	cmd := &certCommand{certPath: "/nonexistent/path.crt", format: "text"}
	err := cmd.run(context.Background(), &bytes.Buffer{})
	require.Error(t, err)

	// Invalid PEM content.
	badPath := filepath.Join(t.TempDir(), "bad.crt")
	require.NoError(t, os.WriteFile(badPath, []byte("not a certificate"), 0600))
	cmd = &certCommand{certPath: badPath, format: "text"}
	err = cmd.run(context.Background(), &bytes.Buffer{})
	require.Error(t, err)
}

func TestCertInspect_EmptyFieldsOmitted(t *testing.T) {
	certPath := generateTestCert(t, tlsca.Identity{
		Username: "minimal@example.com",
		Groups:   []string{"access"},
	})

	cmd := &certCommand{certPath: certPath, format: "text"}
	var buf bytes.Buffer
	err := cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	output := buf.String()

	// These sections should not appear when fields are empty.
	assert.NotContains(t, output, "Route To App:")
	assert.NotContains(t, output, "Route To Database:")
	assert.NotContains(t, output, "Device Extensions:")
	assert.NotContains(t, output, "Pinned IP:")
	assert.NotContains(t, output, "Bot Name:")
}

func TestCertInspect_SubjectFromCert(t *testing.T) {
	// Verify that the X509 header fields come from the certificate itself,
	// not just the identity.
	clock := clockwork.NewFakeClock()
	expires := clock.Now().Add(1 * time.Hour)

	ca, err := tlsca.FromKeys([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	id := tlsca.Identity{
		Username: "certuser",
		Groups:   []string{"admin"},
		Expires:  expires,
	}
	subj, err := id.Subject()
	require.NoError(t, err)

	// Override the common name to verify it's read from the cert.
	subj.CommonName = "certuser"

	certPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  expires,
	})
	require.NoError(t, err)

	certPath := filepath.Join(t.TempDir(), "test.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))

	cmd := &certCommand{certPath: certPath, format: "text"}
	var buf bytes.Buffer
	err = cmd.run(context.Background(), &buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Subject:")
	assert.Contains(t, output, "certuser")
}
