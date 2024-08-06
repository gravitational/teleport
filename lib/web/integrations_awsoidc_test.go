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
	"encoding/base64"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestBuildDeployServiceConfigureIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"deployservice-iam.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"myRole"},
				"taskRole":        []string{"taskRole"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure deployservice-iam " +
				`--cluster=localhost ` +
				`--name=myintegration ` +
				`--aws-region=us-east-1 ` +
				`--role=myRole ` +
				`--task-role=taskRole`,
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"Test+1=2,3.4@5-6_7"},
				"taskRole":        []string{"taskRole"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure deployservice-iam " +
				`--cluster=localhost ` +
				`--name=myintegration ` +
				`--aws-region=us-east-1 ` +
				`--role=Test\+1=2,3.4\@5-6_7 ` +
				`--task-role=taskRole`,
		},
		{
			name: "missing aws-region",
			reqQuery: url.Values{
				"role":            []string{"myRole"},
				"taskRole":        []string{"taskRole"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"taskRole":        []string{"taskRole"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing task role",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"role"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration name",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"role"},
				"taskRole":  []string{"taskRole"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"role"},
				"taskRole":        []string{"taskRole"},
				"integrationName": []string{"'; rm -rf /tmp/dir; echo '"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildEICEConfigureIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"eice-iam.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"myRole"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure eice-iam " +
				"--aws-region=us-east-1 " +
				"--role=myRole",
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"Test+1=2,3.4@5-6_7"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure eice-iam " +
				"--aws-region=us-east-1 " +
				"--role=Test\\+1=2,3.4\\@5-6_7",
		},
		{
			name: "missing aws-region",
			reqQuery: url.Values{
				"role": []string{"myRole"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion": []string{"'; rm -rf /tmp/dir; echo '"},
				"role":      []string{"role"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildEC2SSMIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	proxyPublicURL := env.proxies[0].webURL.String()

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"ec2-ssm-iam.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"myRole"},
				"ssmDocument":     []string{"TeleportDiscoveryInstallerTest"},
				"integrationName": []string{"my-integration"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure ec2-ssm-iam " +
				"--role=myRole " +
				"--aws-region=us-east-1 " +
				"--ssm-document-name=TeleportDiscoveryInstallerTest " +
				"--proxy-public-url=" + proxyPublicURL + " " +
				"--cluster=localhost " +
				"--name=my-integration",
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"Test+1=2,3.4@5-6_7"},
				"ssmDocument":     []string{"TeleportDiscoveryInstallerTest"},
				"integrationName": []string{"my-integration"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure ec2-ssm-iam " +
				"--role=Test\\+1=2,3.4\\@5-6_7 " +
				"--aws-region=us-east-1 " +
				"--ssm-document-name=TeleportDiscoveryInstallerTest " +
				"--proxy-public-url=" + proxyPublicURL + " " +
				"--cluster=localhost " +
				"--name=my-integration",
		},
		{
			name: "missing aws-region",
			reqQuery: url.Values{
				"role":        []string{"myRole"},
				"ssmDocument": []string{"TeleportDiscoveryInstallerTest"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"awsRegion":   []string{"us-east-1"},
				"ssmDocument": []string{"TeleportDiscoveryInstallerTest"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing ssm document",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"myRole"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion":   []string{"'; rm -rf /tmp/dir; echo '"},
				"role":        []string{"role"},
				"ssmDocument": []string{"TeleportDiscoveryInstallerTest"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildAWSAppAccessConfigureIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	anonymousHTTPClient := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"aws-app-access-iam.sh",
	}
	endpoint := anonymousHTTPClient.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"myRole"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure aws-app-access-iam " +
				"--role=myRole",
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"role": []string{"Test+1=2,3.4@5-6_7"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure aws-app-access-iam " +
				"--role=Test\\+1=2,3.4\\@5-6_7",
		},
		{
			name:     "missing role",
			reqQuery: url.Values{},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"role": []string{"'; rm -rf /tmp/dir; echo '"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := anonymousHTTPClient.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildEKSConfigureIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"eks-iam.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"myRole"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure eks-iam " +
				"--aws-region=us-east-1 " +
				"--role=myRole",
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"Test+1=2,3.4@5-6_7"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure eks-iam " +
				"--aws-region=us-east-1 " +
				"--role=Test\\+1=2,3.4\\@5-6_7",
		},
		{
			name: "missing aws-region",
			reqQuery: url.Values{
				"role": []string{"myRole"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion": []string{"'; rm -rf /tmp/dir; echo '"},
				"role":      []string{"role"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildAWSOIDCIdPConfigureScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)
	proxyPublicURL := env.proxies[0].webURL

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"awsoidc-idp.sh",
	}
	scriptEndpoint := publicClt.Endpoint(pathVars...)

	jwksEndpoint := publicClt.Endpoint(".well-known", "jwks-oidc")
	resp, err := publicClt.Get(ctx, jwksEndpoint, nil)
	require.NoError(t, err)
	jwksBase64 := base64.StdEncoding.EncodeToString(resp.Bytes())

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"myRole"},
				"integrationName": []string{"myintegration"},
				"s3Bucket":        []string{"my-bucket"},
				"s3Prefix":        []string{"prefix"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure awsoidc-idp " +
				"--cluster=localhost " +
				"--name=myintegration " +
				"--role=myRole " +
				`--s3-bucket-uri=s3://my-bucket/prefix ` +
				"--s3-jwks-base64=" + jwksBase64,
		},
		{
			name: "valid with proxy endpoint",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"myRole"},
				"integrationName": []string{"myintegration"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure awsoidc-idp " +
				"--cluster=localhost " +
				"--name=myintegration " +
				"--role=myRole " +
				"--proxy-public-url=" + proxyPublicURL.String(),
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"Test+1=2,3.4@5-6_7"},
				"integrationName": []string{"myintegration"},
				"s3Bucket":        []string{"my-bucket"},
				"s3Prefix":        []string{"prefix"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure awsoidc-idp " +
				"--cluster=localhost " +
				"--name=myintegration " +
				"--role=Test\\+1=2,3.4\\@5-6_7 " +
				`--s3-bucket-uri=s3://my-bucket/prefix ` +
				"--s3-jwks-base64=" + jwksBase64,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"integrationName": []string{"myintegration"},
				"s3Bucket":        []string{"my-bucket"},
				"s3Prefix":        []string{"prefix"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing integration name",
			reqQuery: url.Values{
				"role":     []string{"role"},
				"s3Bucket": []string{"my-bucket"},
				"s3Prefix": []string{"prefix"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing s3 bucket",
			reqQuery: url.Values{
				"integrationName": []string{"myintegration"},
				"role":            []string{"role"},
				"s3Prefix":        []string{"prefix"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing s3 prefix",
			reqQuery: url.Values{
				"integrationName": []string{"myintegration"},
				"role":            []string{"role"},
				"s3Bucket":        []string{"my-bucket"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion":       []string{"us-east-1"},
				"role":            []string{"role"},
				"integrationName": []string{"'; rm -rf /tmp/dir; echo '"},
				"s3Bucket":        []string{"my-bucket"},
				"s3Prefix":        []string{"prefix"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, scriptEndpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestBuildListDatabasesConfigureIAMScript(t *testing.T) {
	t.Parallel()
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	ctx := context.Background()
	env := newWebPack(t, 1)

	// Unauthenticated client for script downloading.
	publicClt := env.proxies[0].newClient(t)
	pathVars := []string{
		"webapi",
		"scripts",
		"integrations",
		"configure",
		"listdatabases-iam.sh",
	}
	endpoint := publicClt.Endpoint(pathVars...)

	tests := []struct {
		name                 string
		reqRelativeURL       string
		reqQuery             url.Values
		errCheck             require.ErrorAssertionFunc
		expectedTeleportArgs string
	}{
		{
			name: "valid",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"myRole"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure listdatabases-iam " +
				"--aws-region=us-east-1 " +
				"--role=myRole",
		},
		{
			name: "valid with symbols in role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
				"role":      []string{"Test+1=2,3.4@5-6_7"},
			},
			errCheck: require.NoError,
			expectedTeleportArgs: "integration configure listdatabases-iam " +
				"--aws-region=us-east-1 " +
				"--role=Test\\+1=2,3.4\\@5-6_7",
		},
		{
			name: "missing aws-region",
			reqQuery: url.Values{
				"role": []string{"myRole"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing role",
			reqQuery: url.Values{
				"awsRegion": []string{"us-east-1"},
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "trying to inject escape sequence into query params",
			reqQuery: url.Values{
				"awsRegion": []string{"'; rm -rf /tmp/dir; echo '"},
				"role":      []string{"role"},
			},
			errCheck: isBadParamErrFn,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := publicClt.Get(ctx, endpoint, tc.reqQuery)
			tc.errCheck(t, err)
			if err != nil {
				return
			}

			require.Contains(t, string(resp.Bytes()),
				fmt.Sprintf("teleportArgs='%s'\n", tc.expectedTeleportArgs),
			)
		})
	}
}

func TestAWSOIDCRequiredVPCSHelper(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	clt := env.proxies[0].client

	matchRegion := "us-east-1"
	matchAccountId := "123456789012"
	req := ui.AWSOIDCRequiredVPCSRequest{
		Region:    matchRegion,
		AccountID: matchAccountId,
	}

	upsertDbSvcFn := func(vpcId string, matcher []*types.DatabaseResourceMatcher) {
		if matcher == nil {
			matcher = []*types.DatabaseResourceMatcher{
				{
					Labels: &types.Labels{
						types.DiscoveryLabelAccountID: []string{matchAccountId},
						types.DiscoveryLabelRegion:    []string{matchRegion},
						types.DiscoveryLabelVPCID:     []string{vpcId},
					},
				},
			}
		}
		svc, err := types.NewDatabaseServiceV1(types.Metadata{
			Name:   uuid.NewString(),
			Labels: map[string]string{types.AWSOIDCAgentLabel: types.True},
		}, types.DatabaseServiceSpecV1{
			ResourceMatchers: matcher,
		})
		require.NoError(t, err)
		_, err = env.server.Auth().UpsertDatabaseService(ctx, svc)
		require.NoError(t, err)
	}

	extractKeysFn := func(resp *ui.AWSOIDCRequiredVPCSResponse) []string {
		keys := make([]string, 0, len(resp.VPCMapOfSubnets))
		for k := range resp.VPCMapOfSubnets {
			keys = append(keys, k)
		}
		return keys
	}

	vpcs := []string{"vpc-1", "vpc-2", "vpc-3", "vpc-4", "vpc-5"}
	rdss := []*types.DatabaseV3{}
	for _, vpc := range vpcs {
		rdss = append(rdss,
			mustCreateRDS(t, types.RDS{
				VPCID: vpc,
			}),
		)
	}

	// Double check we start with 0 db svcs.
	s, err := env.server.Auth().ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindDatabaseService,
	})
	require.NoError(t, err)
	require.Empty(t, s.Resources)

	// All vpc's required.
	resp, err := awsOIDCRequiredVPCSHelper(ctx, clt, req, rdss)
	require.NoError(t, err)
	require.Len(t, resp.VPCMapOfSubnets, 5)
	require.ElementsMatch(t, vpcs, extractKeysFn(resp))

	// Insert two valid database services.
	upsertDbSvcFn("vpc-1", nil)
	upsertDbSvcFn("vpc-5", nil)

	// Insert two invalid database services.
	upsertDbSvcFn("vpc-2", []*types.DatabaseResourceMatcher{
		{
			Labels: &types.Labels{
				types.DiscoveryLabelAccountID: []string{matchAccountId},
				types.DiscoveryLabelRegion:    []string{"us-east-2"}, // not matching region
				types.DiscoveryLabelVPCID:     []string{"vpc-2"},
			},
		},
	})
	upsertDbSvcFn("vpc-2a", []*types.DatabaseResourceMatcher{
		{
			Labels: &types.Labels{
				types.DiscoveryLabelAccountID: []string{matchAccountId},
				types.DiscoveryLabelRegion:    []string{matchRegion},
				types.DiscoveryLabelVPCID:     []string{"vpc-2"},
				"something":                   []string{"extra"}, // not matching b/c extra label
			},
		},
	})

	// Double check services were created.
	s, err = env.server.Auth().ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindDatabaseService,
	})
	require.NoError(t, err)
	require.Len(t, s.Resources, 4)

	// Test that only 3 vpcs are required.
	resp, err = awsOIDCRequiredVPCSHelper(ctx, clt, req, rdss)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"vpc-2", "vpc-3", "vpc-4"}, extractKeysFn(resp))

	// Insert the rest of db services
	upsertDbSvcFn("vpc-2", nil)
	upsertDbSvcFn("vpc-3", nil)
	upsertDbSvcFn("vpc-4", nil)

	// Test no required vpcs.
	resp, err = awsOIDCRequiredVPCSHelper(ctx, clt, req, rdss)
	require.NoError(t, err)
	require.Empty(t, resp.VPCMapOfSubnets)
}

func TestAWSOIDCRequiredVPCSHelper_CombinedSubnetsForAVpcID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	clt := env.proxies[0].client

	rdsVPC1 := mustCreateRDS(t, types.RDS{
		VPCID:   "vpc-1",
		Subnets: []string{"subnet1", "subnet2"},
	})

	rdsVPC1a := mustCreateRDS(t, types.RDS{
		VPCID:   "vpc-1",
		Subnets: []string{"subnet2", "subnet3", "subnet4", "subnet1"},
	})

	rdsVPC2 := mustCreateRDS(t, types.RDS{
		VPCID:   "vpc-2",
		Subnets: []string{"subnet8"},
	})

	rdss := []*types.DatabaseV3{rdsVPC1, rdsVPC1a, rdsVPC2}

	resp, err := awsOIDCRequiredVPCSHelper(ctx, clt, ui.AWSOIDCRequiredVPCSRequest{Region: "us-east-1"}, rdss)
	require.NoError(t, err)
	require.Len(t, resp.VPCMapOfSubnets, 2)
	require.ElementsMatch(t, []string{"subnet1", "subnet2", "subnet3", "subnet4"}, resp.VPCMapOfSubnets["vpc-1"])
	require.ElementsMatch(t, []string{"subnet8"}, resp.VPCMapOfSubnets["vpc-2"])
}

func mustCreateRDS(t *testing.T, awsRDS types.RDS) *types.DatabaseV3 {
	rdsDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "x",
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "endpoint.amazonaws.com:5432",
		AWS: types.AWS{
			RDS: awsRDS,
		},
	})
	require.NoError(t, err)
	return rdsDB
}

func TestAWSOIDCSecurityGroupsRulesConverter(t *testing.T) {
	for _, tt := range []struct {
		name     string
		in       []*integrationv1.SecurityGroupRule
		expected []awsoidc.SecurityGroupRule
	}{
		{
			name: "valid",
			in: []*integrationv1.SecurityGroupRule{{
				IpProtocol: "tcp",
				FromPort:   8080,
				ToPort:     8081,
				Cidrs: []*integrationv1.SecurityGroupRuleCIDR{{
					Cidr:        "10.10.10.0/24",
					Description: "cidr x",
				}},
			}},
			expected: []awsoidc.SecurityGroupRule{{
				IPProtocol: "tcp",
				FromPort:   8080,
				ToPort:     8081,
				CIDRs: []awsoidc.CIDR{{
					CIDR:        "10.10.10.0/24",
					Description: "cidr x",
				}},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out := awsOIDCSecurityGroupsRulesConverter(tt.in)
			require.Equal(t, tt.expected, out)
		})
	}
}

func TestAWSOIDCAppAccessAppServerCreationDeletion(t *testing.T) {
	env := newWebPack(t, 1)
	ctx := context.Background()

	roleTokenCRD, err := types.NewRole(services.RoleNameForUser("my-user"), types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{"*": []string{"*"}},
			Rules: []types.Rule{
				types.NewRule(types.KindIntegration, []string{types.VerbRead}),
				types.NewRule(types.KindAppServer, []string{types.VerbCreate, types.VerbUpdate, types.VerbList, types.VerbDelete}),
				types.NewRule(types.KindUserGroup, []string{types.VerbList, types.VerbRead}),
			},
		},
	})
	require.NoError(t, err)

	proxy := env.proxies[0]
	proxyPublicAddr := proxy.handler.handler.cfg.PublicProxyAddr
	pack := proxy.authPack(t, "foo@example.com", []types.Role{roleTokenCRD})

	myIntegration, err := types.NewIntegrationAWSOIDC(types.Metadata{
		Name: "my-integration",
	}, &types.AWSOIDCIntegrationSpecV1{
		RoleARN: "some-arn-role",
	})
	require.NoError(t, err)

	_, err = env.server.Auth().CreateIntegration(ctx, myIntegration)
	require.NoError(t, err)

	// Deleting the AWS App Access should return an error because it was not created yet.
	deleteEndpoint := pack.clt.Endpoint("webapi", "sites", "localhost", "integrations", "aws-oidc", "aws-app-access", "my-integration")
	_, err = pack.clt.Delete(ctx, deleteEndpoint)
	require.Error(t, err)
	require.ErrorContains(t, err, "not found")

	// Create the AWS App Access for the current integration.
	endpoint := pack.clt.Endpoint("webapi", "sites", "localhost", "integrations", "aws-oidc", "my-integration", "aws-app-access")
	_, err = pack.clt.PostJSON(ctx, endpoint, nil)
	require.NoError(t, err)

	// Ensure the AppServer was correctly created.
	appServers, err := env.server.Auth().GetApplicationServers(ctx, "default")
	require.NoError(t, err)
	require.Len(t, appServers, 1)

	expectedServer := &types.AppServerV3{
		Kind:    types.KindAppServer,
		Version: types.V3,
		Metadata: types.Metadata{
			Name: "my-integration",
		},
		Spec: types.AppServerSpecV3{
			Version: api.Version,
			HostID:  "proxy0",
			App: &types.AppV3{
				Kind:    types.KindApp,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: "my-integration",
				},
				Spec: types.AppSpecV3{
					URI:         "https://console.aws.amazon.com",
					Integration: "my-integration",
					Cloud:       "AWS",
					PublicAddr:  "my-integration." + proxyPublicAddr,
				},
			},
		},
	}

	require.Empty(t, cmp.Diff(
		expectedServer,
		appServers[0],
		cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Namespace"),
	))

	// After deleting the application, it should be removed from the backend.
	_, err = pack.clt.Delete(ctx, deleteEndpoint)
	require.NoError(t, err)
	appServers, err = env.server.Auth().GetApplicationServers(ctx, "default")
	require.NoError(t, err)
	require.Empty(t, appServers)
}
