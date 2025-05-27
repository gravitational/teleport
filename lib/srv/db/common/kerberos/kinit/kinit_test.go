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
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
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
