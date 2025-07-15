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

package kerberos

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common/kerberos/kinit"
	"github.com/gravitational/teleport/lib/winpki"
)

//go:embed kinit/testdata/kinit.cache
var fixedCacheData []byte

//go:embed testuser.keytab
var keytabData []byte

// expectedKeytabDataPrefix is result of loading keytabData up until the random SessionID.
const expectedKeytabDataPrefix = `Credentials:
{
  "Username": "alice",
  "DisplayName": "alice",
  "Realm": "example.com",
  "Keytab": true,
  "Password": false,
  "ValidUntil": "0001-01-01T00:00:00Z",
  "Authenticated": false,
  "Human": true,
  "AuthTime": "0001-01-01T00:00:00Z",
`

const (
	mockCA = `-----BEGIN CERTIFICATE-----
MIIECzCCAvOgAwIBAgIRAPEVuzVonTAvpOMyNii7nOAwDQYJKoZIhvcNAQELBQAw
gZ4xNDAyBgNVBAoTK2NlcmVicm8uYWxpc3RhbmlzLmdpdGh1Yi5iZXRhLnRhaWxz
Y2FsZS5uZXQxNDAyBgNVBAMTK2NlcmVicm8uYWxpc3RhbmlzLmdpdGh1Yi5iZXRh
LnRhaWxzY2FsZS5uZXQxMDAuBgNVBAUTJzMyMDQ1Njc4MjI2MDI1ODkyMjc5NTk2
NDc0MTEyOTU0ODMwNzY4MDAeFw0yMjA2MDcwNDQ4MzhaFw0zMjA2MDQwNDQ4Mzha
MIGeMTQwMgYDVQQKEytjZXJlYnJvLmFsaXN0YW5pcy5naXRodWIuYmV0YS50YWls
c2NhbGUubmV0MTQwMgYDVQQDEytjZXJlYnJvLmFsaXN0YW5pcy5naXRodWIuYmV0
YS50YWlsc2NhbGUubmV0MTAwLgYDVQQFEyczMjA0NTY3ODIyNjAyNTg5MjI3OTU5
NjQ3NDExMjk1NDgzMDc2ODAwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIB
AQDJVqHTgx9pdPHCrDJ0UtbZMVL/xhihuR44AY8aqSebJbKc/WrYLIJxqO1q8L4c
B+sfblIMMz/Em1IZ3ZF7AajiJFSn8VfGx5xtxC06YWPY3HfflcuY5kGVWtYl8ReD
7j3FJjNq4Rvv+NoYwmQXYw6Nwu90cWHerDY3G0fQOsjgUAipnTS4+/H36pBakNoK
9pipl3Kb6YVtjdxY6KY0gSy0k8NiRUx8sCpxJOwfUSAvtsGd1tw1388ZfWr2Bl2d
st2H+q1ozLZ3IQXSgSl6s63JmvWpsElg8+nXZKB3CNTIhrOvvyV33Ok5uAQ44nel
vLy5r3o2OguPjvC+SrkHn1avAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBpjAPBgNV
HRMBAf8EBTADAQH/MB0GA1UdDgQWBBR0fa5/2sVguUfn8MHmC7DoFl58fzANBgkq
hkiG9w0BAQsFAAOCAQEAAOEBowwaigoFG3rxM5euIyfax2gWPXN63YF3vd5IN75C
gzimkq9c6MRsvaS053xbRF5NncectmBzTY3WQscJ30+tHD84fA5VQCt//lA+G9gi
g8Co+YPraQe8kbZEcAFceGpWrKjCEwiWlrlM56VfmKmGws21N/PBIb5aO0aEHuWs
HOhXH/n0dKrb7IJcpUh0/w02qiUQ6I0usjGwRlE3xkPyWgEkKUcy+eBrfVVV++8e
HDKyflZ05nt/zvM6W/WIeMI7VMPw/Ryr7iynMqAYAhJhTFKdSwuNLDY8eFbOUnbw
21sZcc/b5g+C9N+0lbFxUUF99bt6jLOVUwpR7LRP2g==
-----END CERTIFICATE-----`

	krb5Conf = `[libdefaults]
 default_realm = example.com
 rdns = false


[realms]
 example.com = {
  kdc = host.example.com
  admin_server = host.example.com
  pkinit_eku_checking = kpServerAuth
  pkinit_kdc_hostname = host.example.com
 }`
)

type mockClientProvider struct{}

func (m *mockClientProvider) CreateClient(ctx context.Context, username string) (*client.Client, error) {
	cfg, err := config.NewFromString(krb5Conf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	credentialsCache := &credentials.CCache{}
	err = credentialsCache.Unmarshal(fixedCacheData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.NewFromCCache(credentialsCache, cfg, client.DisablePAFXFAST(true))
}

func TestConnectorKInitClient(t *testing.T) {
	dir := t.TempDir()
	keytabFile := path.Join(dir, "example.keytab")
	require.NoError(t, os.WriteFile(keytabFile, keytabData, 0600))

	krb5ConfFile := path.Join(dir, "krb5.conf")
	require.NoError(t, os.WriteFile(krb5ConfFile, []byte(krb5Conf), 0600))

	t.Log("keytab:", keytabFile)

	for i, tt := range []struct {
		name           string
		databaseSpec   types.DatabaseSpecV3
		errorMessage   string
		validateClient func(*testing.T, *client.Client)
	}{
		{
			name: "keytab from cached data",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					Domain:     "example.com",
					KeytabFile: keytabFile,
					Krb5File:   krb5ConfFile,
				},
			},
			validateClient: func(t *testing.T, clt *client.Client) {
				var bw bytes.Buffer
				clt.Print(&bw)
				prefix, _, found := strings.Cut(bw.String(), `  "SessionID":`)
				require.True(t, found)
				require.Equal(t, expectedKeytabDataPrefix, prefix)
			},
		},
		{
			name: "keytab without Kerberos config",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					KeytabFile: keytabFile,
				},
			},
			errorMessage: "no Kerberos configuration file provided",
		},
		{
			name: "kinit from cached credentials",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					LDAPCert:    mockCA,
					KDCHostName: "kdc.example.com",
				},
			},
			validateClient: func(t *testing.T, clt *client.Client) {
				var bw bytes.Buffer
				clt.Print(&bw)

				require.Contains(t, bw.String(), `"Username": "chris"`)
				require.Contains(t, bw.String(), `"Realm": "ALISTANIS.EXAMPLE.COM"`)
			},
		},
		{
			name: "invalid AD config",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD:       types.AD{},
			},
			errorMessage: "configuration must have either keytab_file or kdc_host_name and ldap_cert",
		},
		{
			name: "kinit invalid certificate",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					LDAPCert:    "BEGIN CERTIFICATE",
					KDCHostName: "kdc.example.com",
				},
			},
			errorMessage: "invalid certificate was provided via AD configuration",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: fmt.Sprintf("db-%v", i),
			}, tt.databaseSpec)
			require.NoError(t, err)

			databaseUser := "alice"

			mockAuth := struct{ winpki.AuthInterface }{} // dummy implementation: none of the mockAuth methods should actually be called.
			provider := newClientProvider(mockAuth, slog.Default())
			provider.providerFun = func(logger *slog.Logger, auth winpki.AuthInterface, adConfig types.AD) (kinit.ClientProvider, error) {
				return &mockClientProvider{}, nil
			}
			provider.skipLogin = true

			clt, err := provider.GetKerberosClient(context.Background(), database.GetAD(), databaseUser)
			if tt.errorMessage != "" {
				require.ErrorContains(t, err, tt.errorMessage)
				require.Nil(t, clt)
			} else {
				require.NoError(t, err)
				tt.validateClient(t, clt)
			}
		})
	}
}
