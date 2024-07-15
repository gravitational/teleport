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

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestCreateDatabaseRequestParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		desc      string
		req       createDatabaseRequest
		errAssert require.ErrorAssertionFunc
	}{
		{
			desc: "valid general",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "protocol",
				URI:      "uri",
			},
			errAssert: require.NoError,
		},
		{
			desc: "valid aws rds",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "protocol",
				URI:      "uri",
				AWSRDS: &awsRDS{
					ResourceID: "resource-id",
					AccountID:  "account-id",
					Subnets:    []string{"subnet-123", "subnet-321"},
					VPCID:      "vpc-123",
				},
			},
			errAssert: require.NoError,
		},
		{
			desc: "invalid missing name",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing protocol",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "",
				URI:      "uri",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing uri",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "protocol",
				URI:      "",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing aws rds account id",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
				AWSRDS: &awsRDS{
					ResourceID: "resource-id",
					Subnets:    []string{"subnet-123", "subnet-321"},
					VPCID:      "vpc-123",
				},
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing aws rds resource id",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
				AWSRDS: &awsRDS{
					AccountID: "account-id",
					Subnets:   []string{"subnet-123", "subnet-321"},
					VPCID:     "vpc-123",
				},
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing aws rds subnets",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
				AWSRDS: &awsRDS{
					ResourceID: "resource-id",
					AccountID:  "account-id",
					VPCID:      "vpc-123",
				},
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing aws rds vpcid",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
				AWSRDS: &awsRDS{
					ResourceID: "resource-id",
					AccountID:  "account-id",
					Subnets:    []string{"subnet-123", "subnet-321"},
				},
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssert(t, test.req.checkAndSetDefaults())
		})
	}
}

var fakeValidTLSCert = `-----BEGIN CERTIFICATE-----
MIIDyzCCArOgAwIBAgIQD3MiJ2Au8PicJpCNFbvcETANBgkqhkiG9w0BAQsFADBe
MRQwEgYDVQQKEwtleGFtcGxlLmNvbTEUMBIGA1UEAxMLZXhhbXBsZS5jb20xMDAu
BgNVBAUTJzIwNTIxNzE3NzMzMTIxNzQ2ODMyNjA5NjAxODEwODc0NTAzMjg1ODAe
Fw0yMTAyMTcyMDI3MjFaFw0yMTAyMTgwODI4MjFaMIGCMRUwEwYDVQQHEwxhY2Nl
c3MtYWRtaW4xCTAHBgNVBAkTADEYMBYGA1UEEQwPeyJsb2dpbnMiOm51bGx9MRUw
EwYDVQQKEwxhY2Nlc3MtYWRtaW4xFTATBgNVBAMTDGFjY2Vzcy1hZG1pbjEWMBQG
BSvODwEHEwtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAM5FFaCeK59lwIthyXgSCMZbHTDxsy66Cbm/XhwFbKQLngyS0oKkHbh06INN
UfTAAEaFlMG0CzdAyGyRSu9FK8BE127kRHBs6hb1pTgy2f6TFkFo/h4WTWW4GQSi
O8Al7A2tuRjc3mAnk71q+kvpQYS7tnkhmFCYE8jKxMtlYG39x4kQ6btll7P9zI6X
Zv5RRrlzqADuwZpEcLYVi0TjITqPbx3rDZT4l+EmslhaoG+xE5Vu+GYXLlvwB9E/
amfN1Z9Kps4Ob6Jxxse9kjeMir9mwiNkBWVyhH/LETDA9Xa6sTQ2e75MYM7yXJLY
OmBKV4g176Qf1T1ye7a/Ggn4t2UCAwEAAaNgMF4wDgYDVR0PAQH/BAQDAgWgMB0G
A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB8GA1Ud
IwQYMBaAFJWqMooE05nf263F341pOO+mPMSqMA0GCSqGSIb3DQEBCwUAA4IBAQCK
s0yPzkSuCY/LFeHJoJeNJ1SR+EKbk4zoAnD0nbbIsd2quyYIiojshlfehhuZE+8P
bzpUNG2aYKq+8lb0NO+OdZW7kBEDWq7ZwC8OG8oMDrX385fLcicm7GfbGCmZ6286
m1gfG9yqEte7pxv3yWM+7X2bzEjCBds4feahuKPNxOAOSfLUZiTpmOVlRzrpRIhu
2XxiuH+E8n4AP8jf/9bGvKd8PyHohtHVf8HWuKLZxWznQhoKkcfmUmlz5q8ci4Bq
WQdM2NXAMABGAofGrVklPIiraUoHzr0Xxpia4vQwRewYXv8bCPHW+8g8vGBGvoG2
gtLit9DL5DR5ac/CRGJt
-----END CERTIFICATE-----`

func TestUpdateDatabaseRequestParameters(t *testing.T) {
	for _, test := range []struct {
		desc      string
		req       updateDatabaseRequest
		errAssert require.ErrorAssertionFunc
	}{
		{
			desc: "valid",
			req: updateDatabaseRequest{
				CACert: &fakeValidTLSCert,
			},
			errAssert: require.NoError,
		},
		{
			desc: "invalid missing ca_cert",
			req: updateDatabaseRequest{
				CACert: strPtr(""),
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid ca_cert format",
			req: updateDatabaseRequest{
				CACert: strPtr("ca_cert"),
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssert(t, test.req.checkAndSetDefaults())
		})
	}
}

func TestHandleDatabasesGetIAMPolicy(t *testing.T) {
	t.Parallel()

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)

	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-redshift",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5438",
	})
	require.NoError(t, err)

	elasticache, err := types.NewDatabaseV3(types.Metadata{
		Name: "aws-elasticache",
	}, types.DatabaseSpecV3{
		Protocol: "redis",
		URI:      "clustercfg.my-redis-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
		AWS: types.AWS{
			AccountID: "123456789012",
			ElastiCache: types.ElastiCache{
				ReplicationGroupID: "some-group",
			},
		},
	})
	require.NoError(t, err)

	selfHosted, err := types.NewDatabaseV3(types.Metadata{
		Name: "self-hosted",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:12345",
	})
	require.NoError(t, err)

	// Add database servers for above databases.
	for _, db := range []*types.DatabaseV3{redshift, elasticache, selfHosted} {
		_, err = env.server.Auth().UpsertDatabaseServer(context.TODO(), mustCreateDatabaseServer(t, db))
		require.NoError(t, err)
	}

	tests := []struct {
		inputDatabaseName string
		verifyResponse    func(*testing.T, *roundtrip.Response, error)
	}{
		{
			inputDatabaseName: "aws-redshift",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
				requireDatabaseIAMPolicyAWS(t, resp.Bytes(), redshift)
			},
		},
		{
			inputDatabaseName: "aws-elasticache",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.Code())
				requireDatabaseIAMPolicyAWS(t, resp.Bytes(), elasticache)
			},
		},
		{
			inputDatabaseName: "self-hosted",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, http.StatusBadRequest, resp.Code())
			},
		},
		{
			inputDatabaseName: "not-found",
			verifyResponse: func(t *testing.T, resp *roundtrip.Response, err error) {
				require.True(t, trace.IsNotFound(err))
				require.Equal(t, http.StatusNotFound, resp.Code())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.inputDatabaseName, func(t *testing.T) {
			resp, err := pack.clt.Get(context.TODO(), pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databases", test.inputDatabaseName, "iam", "policy"), nil)
			test.verifyResponse(t, resp, err)
		})
	}
}

type listDatabaseServicesResp struct {
	Items []ui.DatabaseService `json:"items"`
}

func TestHandleDatabaseServicesGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := "user"
	roleRODatabaseServices, err := types.NewRole(services.RoleNameForUser(user), types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseServiceLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindDatabaseService,
					[]string{types.VerbRead, types.VerbList}),
			},
		},
	})
	require.NoError(t, err)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, user, []types.Role{roleRODatabaseServices})

	var listDBServicesResp listDatabaseServicesResp

	// No DatabaseServices exist
	resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databaseservices"), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listDBServicesResp))

	require.Empty(t, listDBServicesResp.Items)

	// Adding one DatabaseService
	dbServiceName := uuid.NewString()
	dbService001, err := types.NewDatabaseServiceV1(types.Metadata{
		Name: dbServiceName,
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{"env": []string{"prod"}},
			},
		},
	})
	require.NoError(t, err)

	_, err = env.server.Auth().UpsertDatabaseService(ctx, dbService001)
	require.NoError(t, err)

	// The API returns one DatabaseService.
	resp, err = pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "sites", env.server.ClusterName(), "databaseservices"), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.NoError(t, json.Unmarshal(resp.Bytes(), &listDBServicesResp))

	dbServices := listDBServicesResp.Items
	require.Len(t, dbServices, 1)
	respDBService := dbServices[0]

	require.Equal(t, respDBService.Name, dbServiceName)

	require.Len(t, respDBService.ResourceMatchers, 1)
	respResourceMatcher := respDBService.ResourceMatchers[0]

	require.Equal(t, &types.Labels{"env": []string{"prod"}}, respResourceMatcher.Labels)
}

func TestHandleSQLServerConfigureScript(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)

	for _, tc := range []struct {
		desc        string
		uri         string
		assertError require.ErrorAssertionFunc
		tokenFunc   func(*testing.T) string
	}{
		{
			desc: "valid token and uri",
			uri:  "instance.example.teleport.dev:1433",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.NoError,
		},
		{
			desc: "valid token and empty uri",
			uri:  "",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.Error,
		},
		{
			desc: "valid token and invalid uri",
			uri:  "hello#hello",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.Error,
		},
		{
			desc: "invalid line break character token and invalid uri",
			uri:  "computer.domain\n.com:1433",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.Error,
		},
		{
			desc: "invalid character ` token and invalid uri",
			uri:  "computer.domain`.com:1433",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.Error,
		},
		{
			desc: "invalid character | token and invalid uri",
			uri:  "computer.domain|.com:1433",
			tokenFunc: func(t *testing.T) string {
				pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
				require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
				return token
			},
			assertError: require.Error,
		},
		{
			desc:        "invalid token",
			uri:         "instance.example.teleport.dev:1433",
			tokenFunc:   func(_ *testing.T) string { return "random-token" },
			assertError: require.Error,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := pack.clt.Get(
				ctx,
				pack.clt.Endpoint("webapi/scripts/databases/configure/sqlserver", tc.tokenFunc(t), "configure-ad.ps1"),
				url.Values{"uri": []string{tc.uri}},
			)
			tc.assertError(t, err)
		})
	}

}

// TestHandleSQLServerConfigureScriptDatabaseURIEscaped given a SQL Server
// database URI, ensures that special characters are escaped when placed on the
// PowerShell script.
func TestHandleSQLServerConfigureScriptDatabaseURIEscaped(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, "user", nil /* roles */)
	pt, token := generateProvisionToken(t, types.RoleDatabase, env.clock.Now().Add(time.Hour))
	require.NoError(t, env.server.Auth().CreateToken(ctx, pt))
	re := regexp.MustCompile(`\$DB_ADDRESS\s*=\s*'([^']+)'`)

	for _, c := range []string{";", "\"", "'", "&", "$", "(", ")"} {
		t.Run(c, func(t *testing.T) {
			uri := fmt.Sprintf("database.ad%s.com:1433", c)
			resp, err := pack.clt.Get(
				ctx,
				pack.clt.Endpoint("webapi/scripts/databases/configure/sqlserver", token, "configure-ad.ps1"),
				url.Values{"uri": []string{uri}},
			)
			require.NoError(t, err)
			escapedURIResult := re.FindStringSubmatch(string(resp.Bytes()))
			require.Len(t, escapedURIResult, 2)
			require.NotEqual(t, uri, escapedURIResult[1])
			require.Contains(t, escapedURIResult[1], url.QueryEscape(c))
		})
	}
}

func mustCreateDatabaseServer(t *testing.T, db *types.DatabaseV3) types.DatabaseServer {
	t.Helper()

	databaseServer, err := types.NewDatabaseServerV3(types.Metadata{
		Name: db.GetName(),
	}, types.DatabaseServerSpecV3{
		HostID:   "host-id",
		Hostname: "host-name",
		Database: db,
	})
	require.NoError(t, err)
	return databaseServer
}

func requireDatabaseIAMPolicyAWS(t *testing.T, respBody []byte, database types.Database) {
	t.Helper()

	var resp databaseIAMPolicyResponse
	require.NoError(t, json.Unmarshal(respBody, &resp))
	require.Equal(t, "aws", resp.Type)

	actualPolicyDocument, err := awslib.ParsePolicyDocument(resp.AWS.PolicyDocument)
	require.NoError(t, err)

	expectedPolicyDocument, expectedPlaceholders, err := dbiam.GetAWSPolicyDocument(database)
	require.NoError(t, err)
	require.Equal(t, expectedPolicyDocument, actualPolicyDocument)
	require.Equal(t, []string(expectedPlaceholders), resp.AWS.Placeholders)
}

func strPtr(str string) *string {
	return &str
}

func generateProvisionToken(t *testing.T, role types.SystemRole, expiresAt time.Time) (types.ProvisionToken, string) {
	t.Helper()

	token, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)

	pt, err := types.NewProvisionToken(token, types.SystemRoles{role}, expiresAt)
	require.NoError(t, err)

	return pt, token
}
