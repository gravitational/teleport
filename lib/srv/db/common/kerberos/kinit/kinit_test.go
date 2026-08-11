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

package kinit

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/pem"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/fixtures"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/winpki"
)

//go:embed testdata/kinit.cache
var validCacheData []byte
var badCacheData = []byte("bad cache data to write to file")

type fixedCacheCommandRunner struct {
	cacheData    []byte
	errorMessage string
}

func (f *fixedCacheCommandRunner) runCommand(ctx context.Context, env map[string]string, command string, args ...string) (string, error) {
	if f.errorMessage != "" {
		return "", trace.BadParameter("error: %s", f.errorMessage)
	}

	if len(args) != 8 {
		return "", trace.BadParameter("unexpected number of arguments %v, wanted 8", len(args))
	}

	// kinit arguments looks like this:
	// ... "-c" <cachePath> "--" <principal>
	if args[4] != "-c" {
		return "", trace.BadParameter("unexpected 5th argument: %v, wanted -c", args[4])
	}
	if args[6] != "--" {
		return "", trace.BadParameter("unexpected 7th argument: %v, wanted --", args[6])
	}

	cachePath := args[5]
	err := os.WriteFile(cachePath, f.cacheData, 0600)
	if err != nil {
		return "failed to write to cache file at " + cachePath, trace.Wrap(err)
	}
	return "returning after having written cache file " + cachePath, nil
}

type testCertGetter struct {
	pass bool
}

func (t *testCertGetter) getCertificate(_ context.Context, username string) (*getCertificateResult, error) {
	if t.pass {
		return &getCertificateResult{}, nil
	}
	return nil, trace.BadParameter("predefined failure to get cert bytes")

}

func TestUseOrCreateCredentials(t *testing.T) {
	for _, tt := range []struct {
		name           string
		commandRunner  *fixedCacheCommandRunner
		certGetter     *testCertGetter
		wantErrMessage string
	}{
		{
			name:          "valid cache file, cert request success",
			commandRunner: &fixedCacheCommandRunner{cacheData: validCacheData},
			certGetter:    &testCertGetter{pass: true},
		},
		{
			name:           "valid cache file, cert request failure",
			commandRunner:  &fixedCacheCommandRunner{cacheData: validCacheData},
			certGetter:     &testCertGetter{pass: false},
			wantErrMessage: "predefined failure to get cert bytes",
		},
		{
			name:           "failure creating cache",
			commandRunner:  &fixedCacheCommandRunner{errorMessage: "test error"},
			certGetter:     &testCertGetter{pass: true},
			wantErrMessage: "test error",
		},
		{
			name:           "invalid cache file",
			commandRunner:  &fixedCacheCommandRunner{cacheData: badCacheData},
			certGetter:     &testCertGetter{pass: true},
			wantErrMessage: "Invalid credential cache data.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth := struct{ winpki.AuthInterface }{}
			provider, err := newKinitProvider(
				nil, auth, types.AD{
					Domain:                 "example.com",
					KDCHostName:            "host.example.com",
					LDAPCert:               fixtures.TLSCACertPEM,
					LDAPServiceAccountName: "DOMAIN\\test-user",
					LDAPServiceAccountSID:  "S-1-5-21-2191801808-3167526388-2669316733-1104",
				})
			require.NoError(t, err)
			provider.certGetter = tt.certGetter
			provider.runner = tt.commandRunner

			clt, err := provider.CreateClient(context.Background(), "alice")
			if tt.wantErrMessage == "" {
				require.NoError(t, err)
				require.NotNil(t, clt)
			} else {
				require.ErrorContains(t, err, tt.wantErrMessage)
				require.Nil(t, clt)
			}
		})
	}
}

// TestKinitProvider_CreateClient_multipleUserCAs tests a bug where having
// multiple caCerts would generate an invalid userca.pem file.
func TestKinitProvider_CreateClient_multipleUserCAs(t *testing.T) {
	t.Parallel()

	const chainLength = 3
	caChain, err := subcaenv.MakeCAChain(chainLength, nil)
	require.NoError(t, err)

	wantCADERs := make([][]byte, 0, chainLength) // intermediates + ldapCert
	caCerts := make([][]byte, 0, chainLength-1)
	for _, ca := range caChain[1:] {
		caCerts = append(caCerts, bytes.TrimSpace(ca.CertPEM))
		wantCADERs = append(wantCADERs, ca.Cert.Raw)
	}

	ldapCertPEM := fixtures.TLSCACertPEM
	ldapCert, err := tlsutils.ParseCertificatePEM([]byte(ldapCertPEM))
	require.NoError(t, err)
	wantCADERs = append(wantCADERs, ldapCert.Raw)

	auth := struct{ winpki.AuthInterface }{}
	provider, err := newKinitProvider(
		logtest.NewLogger(),
		auth, // Auth is skipped by the faked certGetter.
		types.AD{
			// LDAPCert must be present (and valid) for the test to work.
			// Other values just need to satisfy newKinitProvider.
			Domain:                 "example.com",
			KDCHostName:            "host.example.com",
			LDAPCert:               ldapCertPEM,
			LDAPServiceAccountName: "DOMAIN\\test-user",
			LDAPServiceAccountSID:  "S-1-5-21-2191801808-3167526388-2669316733-1104",
		})
	require.NoError(t, err)
	provider.certGetter = &fakeCertGetter{
		result: &getCertificateResult{
			certPEM: []byte(`insert cert pem here`),
			keyPEM:  []byte(`insert key pem here`),
			caCerts: caCerts,
		},
	}
	runner := &parseUserCAsRunner{}
	provider.runner = runner

	const username = "alice"
	_, err = provider.CreateClient(context.Background(), username)
	require.NoError(t, err)

	// Verify anchors.
	if diff := cmp.Diff(wantCADERs, runner.anchorsDER); diff != "" {
		t.Errorf("Method mismatch (-want +got)\n%s", diff)
	}
}

type fakeCertGetter struct {
	result *getCertificateResult
	err    error
}

func (f *fakeCertGetter) getCertificate(ctx context.Context, username string) (*getCertificateResult, error) {
	return f.result, f.err
}

type stringsValue []string

func (s *stringsValue) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (s *stringsValue) String() string {
	return strings.Join(*s, " ")
}

type parseUserCAsRunner struct {
	anchorsDER [][]byte
}

func (r *parseUserCAsRunner) runCommand(
	ctx context.Context,
	env map[string]string,
	command string,
	args ...string,
) (string, error) {
	var xFlag stringsValue
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	cachePath := fs.String("c", "", "")
	fs.Var(&xFlag, "X", "")

	if err := fs.Parse(args); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	const anchorsPrefix = "X509_anchors=FILE:"
	var anchorsFile string
	for _, x := range xFlag {
		if strings.HasPrefix(x, anchorsPrefix) {
			anchorsFile = x[len(anchorsPrefix):]
			break
		}
	}
	if anchorsFile == "" {
		return "", fmt.Errorf("anchors not informed (-X %spath", anchorsPrefix)
	}
	anchorsPEM, err := os.ReadFile(anchorsFile)
	if err != nil {
		return "", fmt.Errorf("read anchors: %w", err)
	}

	// Do an explicit check for the improper concatenation.
	const badLine = "-----END CERTIFICATE----------BEGIN CERTIFICATE-----\n"
	for line := range bytes.Lines(anchorsPEM) {
		if string(line) == badLine {
			return "", fmt.Errorf("found poorly concatenated PEMs in anchors file, data=[%s]", anchorsPEM)
		}
	}
	// Parse anchors PEMs.
	for pems := anchorsPEM; true; {
		block, rest := pem.Decode(pems)
		if block == nil {
			return "", fmt.Errorf("failed to decode PEM, data=[%s]", rest)
		}
		r.anchorsDER = append(r.anchorsDER, block.Bytes)
		pems = rest
		if len(pems) == 0 {
			break
		}
	}

	if err := os.WriteFile(*cachePath, validCacheData, 0600); err != nil {
		return "", fmt.Errorf("write cache data: %w", err)
	}

	return "", nil
}

const (
	expectedConfString = `[libdefaults]
 default_realm = EXAMPLE.COM
 rdns = false


[realms]
 EXAMPLE.COM = {
  kdc = example.com
  admin_server = example.com
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = instance.host.example.com
 }`
)

func TestKRBConfString(t *testing.T) {
	cfg := types.AD{
		Domain:      "example.com",
		KDCHostName: "instance.host.example.com",
	}

	krb5Config, err := newKrb5Config(cfg)
	require.NoError(t, err)
	require.Equal(t, expectedConfString, krb5Config)
}

// TestKRBConfIgnoresLDAPOverrides verifies the LDAP endpoint overrides don't
// leak into the Kerberos configuration: pkinit_kdc_hostname must keep pointing
// at the KDC regardless of where LDAP lives.
func TestKRBConfIgnoresLDAPOverrides(t *testing.T) {
	cfg := types.AD{
		Domain:            "example.com",
		KDCHostName:       "instance.host.example.com",
		LDAPHost:          "ldap.example.com",
		LDAPTLSServerName: "ldap.tls.example.com",
	}

	krb5Config, err := newKrb5Config(cfg)
	require.NoError(t, err)
	require.Equal(t, expectedConfString, krb5Config)
}

type mockConnector struct {
}

func (m *mockConnector) GetActiveDirectorySID(ctx context.Context, username string) (sid string, err error) {
	return "S-1-5-21-2191801808-3167526388-2669316733-1104", nil
}

func TestGetCertificate(t *testing.T) {
	for _, tt := range []struct {
		name          string
		domain        string
		pkiDomain     string
		wantCRLDomain string
	}{
		{
			name:          "CRL domain defaults to domain",
			domain:        "example.com",
			wantCRLDomain: "example.com",
		},
		{
			name:          "pki_domain overrides CRL domain",
			domain:        "child.example.com",
			pkiDomain:     "example.com",
			wantCRLDomain: "example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth := &mockAuthClient{
				generateDatabaseCert: func(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
					require.Equal(t, tt.wantCRLDomain, request.CRLDomain)

					csr, err := tlsca.ParseCertificateRequestPEM(request.CSR)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					require.Equal(t, "CN=alice", csr.Subject.String())
					require.Len(t, csr.Extensions, 3)
					return generateDatabaseCert(ctx, request)
				},
			}

			getter := &dbCertGetter{
				logger:        slog.New(slog.DiscardHandler),
				auth:          auth,
				domain:        tt.domain,
				pkiDomain:     tt.pkiDomain,
				ldapConnector: &mockConnector{},
			}

			_, err := getter.getCertificate(context.Background(), "alice")
			require.NoError(t, err)
		})
	}
}
