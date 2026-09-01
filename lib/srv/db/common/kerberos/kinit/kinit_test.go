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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/cryptosuites"
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

// TestKinitProvider_CreateClient_caOverride tests kinit behavior with a typical
// CA override certificate response.
//
// Tests regressions for:
//   - Invalid PEM concatenation.
//   - kinit "own certificate" validation.
func TestKinitProvider_CreateClient_caOverride(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	const clusterName = "zarquon"
	const username = "alice"

	// External chain:
	// - Root
	// - Int1
	const chainLength = 2
	externalChain, err := subcaenv.MakeCAChain(chainLength, &subcaenv.CAParams{
		Clock: clock,
		ModifyCertificate: func(template *x509.Certificate) {
			template.MaxPathLen += 1
		},
	})
	require.NoError(t, err)
	root := externalChain[0]
	int1 := externalChain[1]

	// Simulate 2 CA overrides (HSM-enabled cluster).
	// - Chain: Root -> Int1 -> Override1
	// - Chain: Root -> Int1 -> Override2
	override1, err := int1.NewIntermediateCA(&subcaenv.CAParams{
		Clock: clock,
		Template: &x509.Certificate{
			Subject: pkix.Name{
				CommonName:   "override1",
				Organization: []string{clusterName},
			},
		},
	})
	require.NoError(t, err)
	override2, err := int1.NewIntermediateCA(&subcaenv.CAParams{
		Clock: clock,
		Template: &x509.Certificate{
			Subject: pkix.Name{
				CommonName:   "override2",
				Organization: []string{clusterName},
			},
		},
	})
	require.NoError(t, err)

	// Mint a client certificate from override1.
	priv, err := cryptosuites.GeneratePrivateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)
	// This is a rather simplified certificate.
	// What matters here is that the trust chain is valid.
	certDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{clusterName},
			CommonName:   username,
		},
		NotBefore: clock.Now().Add(-1 * time.Minute),
		NotAfter:  clock.Now().Add(1 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
	}, override1.Cert, priv.Public(), override1.Key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv.Signer.(*rsa.PrivateKey)),
	})

	ldapCertPEM := fixtures.TLSCACertPEM

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
			DatabaseCredentialsResponse: winpki.DatabaseCredentialsResponse{
				CertPEM: certPEM,
				KeyPEM:  keyPEM,
				CACertsPEM: [][]byte{
					override1.CertPEM,
					override2.CertPEM,
				},
				TrustChainPEM: [][]byte{
					// Leaf-to-root, including the issuing CA override.
					override1.CertPEM,
					int1.CertPEM,
					root.CertPEM,
				},
			},
		},
	}
	runner := &validateChainRunner{
		clock: clock,
	}
	provider.runner = runner

	_, err = provider.CreateClient(context.Background(), username)
	require.NoError(t, err)

	ldapCert, err := tlsutils.ParseCertificatePEM([]byte(ldapCertPEM))
	require.NoError(t, err)

	// Verify anchors.
	wantAnchors := []string{
		// Trust chain (roots).
		root.Cert.Subject.String(),
		// LDAP.
		ldapCert.Subject.String(),
	}
	if diff := cmp.Diff(wantAnchors, runner.anchorNames); diff != "" {
		t.Errorf("kinit X509_anchors mismatch (-want +got)\n%s", diff)
	}

	// Verify pools.
	wantPools := []string{
		// CAs (intermediates).
		override1.Cert.Subject.String(),
		override2.Cert.Subject.String(),
		// Trust chain (intermediates).
		int1.Cert.Subject.String(),
	}
	if diff := cmp.Diff(wantPools, runner.poolNames); diff != "" {
		t.Errorf("kinit pkinit_pool mismatch (-want +got)\n%s", diff)
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

type validateChainRunner struct {
	clock clockwork.Clock

	anchorNames []string // aka cert.Subject.String()
	poolNames   []string // aka cert.Subject.String()
}

func (r *validateChainRunner) runCommand(
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

	krb5ConfigFile := env["KRB5_CONFIG"]
	if krb5ConfigFile == "" {
		return "", errors.New("KRB5_CONFIG missing or empty")
	}
	krb5ConfigBytes, err := os.ReadFile(krb5ConfigFile)
	if err != nil {
		return "", fmt.Errorf("read KRB5_CONFIG: %w", err)
	}
	krb5Config := string(krb5ConfigBytes)
	// Do a superficial config parse.
	if _, err := config.NewFromString(krb5Config); err != nil {
		return "", fmt.Errorf("invalid KRB5_CONFIG: %w", err)
	}

	// Find out if a pkinit_pool file is specified.
	// gokrb5/config.Config doesn't include pkinit_pool, so we'll look for the
	// string.
	const poolPrefix = "pkinit_pool = FILE:"
	var poolFile string
	for line := range strings.Lines(krb5Config) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, poolPrefix) {
			poolFile = line[len(poolPrefix):]
			break
		}
	}
	var poolCAs []byte
	if poolFile != "" {
		var err error
		poolCAs, err = os.ReadFile(poolFile)
		if err != nil {
			return "", fmt.Errorf("read pkinit_pool: %w", err)
		}
	}

	const anchorsPrefix = "X509_anchors=FILE:"
	const identityPrefix = "X509_user_identity=FILE:"
	var anchorsFile string
	var certFile, keyFile string
	for _, x := range xFlag {
		switch {
		case strings.HasPrefix(x, anchorsPrefix):
			anchorsFile = x[len(anchorsPrefix):]

		case strings.HasPrefix(x, identityPrefix):
			x = x[len(identityPrefix):]
			tmp := strings.Split(x, ",")
			if len(tmp) != 2 {
				return "", fmt.Errorf("invalid user identity format: %q", x)
			}
			certFile = tmp[0]
			keyFile = tmp[1]
		}
	}
	switch {
	case anchorsFile == "":
		return "", fmt.Errorf("anchors not informed (-X %spath)", anchorsPrefix)
	case certFile == "":
		return "", errors.New("user_identity certificate not informed")
	case keyFile == "":
		return "", errors.New("user_identity private key not informed")
	}

	anchorsPEM, err := os.ReadFile(anchorsFile)
	if err != nil {
		return "", fmt.Errorf("read anchors: %w", err)
	}
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return "", fmt.Errorf("read user_identity certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return "", fmt.Errorf("read user_identity private key: %w", err)
	}

	anchorPool, anchorNames, err := parseCAs(anchorsPEM)
	if err != nil {
		return "", fmt.Errorf("parse anchors: %w", err)
	}
	r.anchorNames = anchorNames

	poolPool, poolNames, err := parseCAs(poolCAs)
	if err != nil {
		return "", fmt.Errorf("parse pkinit_pool: %w", err)
	}
	r.poolNames = poolNames

	// Parse and validate certificate/key.
	identity, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return "", fmt.Errorf("parse user_identity: %w", err)
	}
	if l := len(identity.Certificate); l > 1 {
		return "", fmt.Errorf("found %d user_identity certificates, expected 1", l)
	}

	// Verify own certificate.
	if _, err := identity.Leaf.Verify(x509.VerifyOptions{
		Intermediates: poolPool,
		Roots:         anchorPool,
		CurrentTime:   r.clock.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return "", fmt.Errorf("failed to verify own certificate: %w", err)
	}

	if err := os.WriteFile(*cachePath, validCacheData, 0600); err != nil {
		return "", fmt.Errorf("write cache data: %w", err)
	}

	return "", nil
}

func parseCAs(casPEM []byte) (_ *x509.CertPool, subjects []string, _ error) {
	if len(casPEM) == 0 {
		return nil, nil, nil
	}

	// Do an explicit check for the improper concatenation.
	const badLine = "-----END CERTIFICATE----------BEGIN CERTIFICATE-----\n"
	for line := range bytes.Lines(casPEM) {
		if string(line) == badLine {
			return nil, nil, fmt.Errorf("found poorly concatenated PEMs in anchors file, data=[%s]", casPEM)
		}
	}

	pool := x509.NewCertPool()

	// Parse CAs.
	for pems := casPEM; true; {
		block, rest := pem.Decode(pems)
		if block == nil {
			return nil, nil, fmt.Errorf("failed to decode PEM, data=[%s]", rest)
		}
		pems = rest

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse anchor certificate: %w", err)
		}
		subjects = append(subjects, cert.Subject.String())
		pool.AddCert(cert)

		if len(pems) == 0 {
			break
		}
	}
	return pool, subjects, nil
}

func TestKRBConfString(t *testing.T) {
	t.Parallel()

	const (
		expectedNoPool = `[libdefaults]
 default_realm = EXAMPLE.COM
 rdns = false


[realms]
 EXAMPLE.COM = {
  kdc = example.com
  admin_server = example.com
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = instance.host.example.com
 }`

		expectedWithPool = `[libdefaults]
 default_realm = EXAMPLE.COM
 rdns = false


[realms]
 EXAMPLE.COM = {
  kdc = example.com
  admin_server = example.com
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = instance.host.example.com
  pkinit_pool = FILE:/path/to/intermediates.pem
 }`
	)

	cfg := types.AD{
		Domain:      "example.com",
		KDCHostName: "instance.host.example.com",
	}

	tests := []struct {
		name     string
		poolPath string
		hasBool  bool
		want     string
	}{
		{
			name: "no pool",
			want: expectedNoPool,
		},
		{
			name:     "with pool",
			poolPath: "/path/to/intermediates.pem",
			hasBool:  true,
			want:     expectedWithPool,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := newKrb5Config(cfg, test.poolPath, test.hasBool)
			require.NoError(t, err)

			if diff := cmp.Diff(test.want, string(got)); diff != "" {
				t.Errorf("newKrb5Config mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

type mockConnector struct {
}

func (m *mockConnector) GetActiveDirectorySID(ctx context.Context, username string) (sid string, err error) {
	return "S-1-5-21-2191801808-3167526388-2669316733-1104", nil
}

func TestGetCertificate(t *testing.T) {
	t.Parallel()
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

			_, err := getter.getCertificate(t.Context(), "alice")
			require.NoError(t, err)
		})
	}
}
