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
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

//go:embed kinit/testdata/kinit.cache
var cacheData []byte

type staticCache struct {
	t    *testing.T
	pass bool
}

func (s *staticCache) CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cachePath := args[len(args)-1]
	require.NotEmpty(s.t, cachePath)
	err := os.WriteFile(cachePath, cacheData, 0664)
	require.NoError(s.t, err)

	if s.pass {
		return exec.Command("echo")
	}
	cmd := exec.Command("")
	cmd.Err = errors.New("bad command")
	return cmd
}

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

type mockAuth struct{}

func (m *mockAuth) GenerateWindowsDesktopCert(ctx context.Context, request *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	return nil, nil
}

func (m *mockAuth) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return nil, nil
}

func (m *mockAuth) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "TestCluster",
		ClusterID:   "TestClusterID",
	})
}

func (m *mockAuth) GenerateDatabaseCert(_ context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	if req.GetRequesterName() != proto.DatabaseCertRequest_UNSPECIFIED {
		return nil, trace.BadParameter("db agent should not specify requester name")
	}
	return &proto.DatabaseCertResponse{Cert: []byte(mockCA), CACerts: [][]byte{[]byte(mockCA)}}, nil
}

func TestConnectorKInitClient(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dir := t.TempDir()

	provider := newClientProvider(&mockAuth{}, dir)
	provider.kinitCommandGenerator = &staticCache{t: t, pass: true}

	krbConfPath := filepath.Join(dir, "krb5.conf")
	err := os.WriteFile(krbConfPath, []byte(krb5Conf), 0664)
	require.NoError(t, err)

	for i, tt := range []struct {
		desc         string
		databaseSpec types.DatabaseSpecV3
		errAssertion require.ErrorAssertionFunc
	}{
		{
			desc: "AD-x509-Loads_and_fails_with_expired_cache",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					LDAPCert:    mockCA,
					KDCHostName: "kdc.example.com",
					Krb5File:    krbConfPath,
				},
			},
			// When using a non-Azure database, the connector should attempt to get a kinit client
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				// we can't get a new TGT without an actual kerberos implementation, so we are relying on the existing
				// credentials cache being expired
				require.ErrorContains(t, err, "cannot login, no user credentials available and no valid existing session")
			},
		},
		{
			desc: "AD-x509-Fails_to_load_with_bad_config",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD:       types.AD{},
			},
			// When using a non-Azure database, the connector should attempt to get a kinit client
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				// we can't get a new TGT without an actual kerberos implementation, so we are relying on the existing
				// credentials cache being expired
				require.ErrorIs(t, err, errBadKerberosConfig)
			},
		},
		{
			desc: "AD-x509-Fails_with_invalid_certificate",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
				AD: types.AD{
					LDAPCert:    "BEGIN CERTIFICATE",
					KDCHostName: "kdc.example.com",
					Krb5File:    krbConfPath,
				},
			},
			// When using a non-Azure database, the connector should attempt to get a kinit client
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				// we can't get a new TGT without an actual kerberos implementation, so we are relying on the existing
				// credentials cache being expired
				require.ErrorIs(t, err, errBadCertificate)
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: fmt.Sprintf("db-%v", i),
			}, tt.databaseSpec)
			require.NoError(t, err)

			databaseUser := "alice"

			session := &common.Session{
				Database:     database,
				DatabaseUser: databaseUser,
				DatabaseName: database.GetName(),
			}

			client, err := provider.GetKerberosClient(ctx, session)
			if client == nil {
				tt.errAssertion(t, err)
			} else {
				err = client.Login()
				tt.errAssertion(t, err)
			}
		})
	}
}
