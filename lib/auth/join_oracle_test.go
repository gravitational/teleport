// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/fixtures"
)

func TestCheckHeaders(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	defaultChallenge := "abcd1234"
	defaultAuthHeader := `Signature headers="x-date x-teleport-challenge"`

	t.Run("ok", func(t *testing.T) {
		headers := formatHeaderFromMap(map[string]string{
			"Authorization":        defaultAuthHeader,
			oracle.DateHeader:      clock.Now().UTC().Format(http.TimeFormat),
			oracle.ChallengeHeader: defaultChallenge,
		})
		require.NoError(t, checkHeaders(headers, defaultChallenge, clock))
	})

	tests := []struct {
		name    string
		headers map[string]string
	}{
		{
			name: "missing signed headers",
			headers: map[string]string{
				"Authorization":        `Signature foo="bar"`,
				oracle.DateHeader:      clock.Now().UTC().Format(http.TimeFormat),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "date not signed",
			headers: map[string]string{
				"Authorization":        `Signature headers="x-teleport-challenge"`,
				oracle.DateHeader:      clock.Now().UTC().Format(http.TimeFormat),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "challenge not signed",
			headers: map[string]string{
				"Authorization":        `Signature headers="x-date"`,
				oracle.DateHeader:      clock.Now().UTC().Format(http.TimeFormat),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "missing date",
			headers: map[string]string{
				"Authorization":        defaultAuthHeader,
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "date too early",
			headers: map[string]string{
				"Authorization":        defaultAuthHeader,
				oracle.DateHeader:      clock.Now().Add(-10 * time.Minute).UTC().Format(http.TimeFormat),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "missing challenge",
			headers: map[string]string{
				"Authorization":   defaultAuthHeader,
				oracle.DateHeader: clock.Now().UTC().Format(http.TimeFormat),
			},
		},
		{
			name: "challenge does not match",
			headers: map[string]string{
				"Authorization":        defaultAuthHeader,
				oracle.DateHeader:      clock.Now().UTC().Format(http.TimeFormat),
				oracle.ChallengeHeader: "differentchallenge",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, checkHeaders(formatHeaderFromMap(tc.headers), defaultChallenge, clock))
		})
	}
}

func makeOCID(resourceType, region, id string) string {
	return fmt.Sprintf("ocid1.%s.oc1.%s.%s", resourceType, region, id)
}

func makeTenancyID(id string) string {
	return makeOCID("tenancy", "", id)
}

func makeCompartmentID(id string) string {
	return makeOCID("compartment", "", id)
}

func makeInstanceID(region, id string) string {
	return makeOCID("instance", region, id)
}

func TestCheckOracleAllowRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		claims     oracle.Claims
		allowRules []*types.ProvisionTokenSpecV2Oracle_Rule
		assert     require.ErrorAssertionFunc
	}{
		{
			name: "ok",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with compartment wildcard",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("foo"),
					Regions: []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region wildcard",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region abbreviation in id",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("phx", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "ok with region abbreviation in token",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"phx"},
				},
			},
			assert: require.NoError,
		},
		{
			name: "wrong tenancy",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("something-else"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.Error,
		},
		{
			name: "wrong compartment",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("something-else"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.Error,
		},
		{
			name: "wrong region",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-ashburn-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy:            makeTenancyID("foo"),
					ParentCompartments: []string{makeCompartmentID("bar")},
					Regions:            []string{"us-phoenix-1"},
				},
			},
			assert: require.Error,
		},
		{
			name: "block match across rules",
			claims: oracle.Claims{
				TenancyID:     makeTenancyID("foo"),
				CompartmentID: makeCompartmentID("bar"),
				InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
			},
			allowRules: []*types.ProvisionTokenSpecV2Oracle_Rule{
				{
					Tenancy: makeTenancyID("foo"),
					Regions: []string{"us-ashburn-1"},
				},
				{
					Tenancy: makeTenancyID("something-else"),
				},
			},
			assert: require.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, checkOracleAllowRules(tc.claims, "mytoken", tc.allowRules))
		})
	}
}

func mapFromHeader(header http.Header) map[string]string {
	out := make(map[string]string, len(header))
	for k := range header {
		out[k] = header.Get(k)
	}
	return out
}

func TestRegisterUsingOracleMethod(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	pemBytes, ok := fixtures.PEMBytes["rsa"]
	require.True(t, ok)

	provider := common.NewRawConfigurationProvider(
		"ocid1.tenancy.oc1..abcd1234",
		"ocid1.user.oc1..abcd1234",
		"us-ashburn-1",
		"fingerprint",
		string(pemBytes),
		nil,
	)

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	token, err := types.NewProvisionTokenFromSpec(
		"my-token",
		time.Now().Add(time.Minute),
		types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodOracle,
			Roles:      []types.SystemRole{types.RoleNode},
			Oracle: &types.ProvisionTokenSpecV2Oracle{
				Allow: []*types.ProvisionTokenSpecV2Oracle_Rule{
					{
						Tenancy: makeTenancyID("foo"),
					},
				},
			},
		},
	)
	require.NoError(t, err)
	require.NoError(t, a.UpsertToken(ctx, token))

	wrongToken, err := types.NewProvisionToken(
		"wrong-token",
		[]types.SystemRole{types.RoleNode},
		time.Now().Add(time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, a.UpsertToken(ctx, wrongToken))

	mockFetchClaims := func(_ context.Context, _ *http.Request) (oracle.Claims, error) {
		return oracle.Claims{
			TenancyID:     makeTenancyID("foo"),
			CompartmentID: makeCompartmentID("bar"),
			InstanceID:    makeInstanceID("us-phoenix-1", "baz"),
		}, nil
	}

	tests := []struct {
		name                string
		modifyTokenRequest  func(*types.RegisterUsingTokenRequest)
		modifyOracleRequest func(*proto.OracleSignedRequest)
		assert              require.ErrorAssertionFunc
	}{
		{
			name:   "ok",
			assert: require.NoError,
		},
		{
			name: "token not found",
			modifyTokenRequest: func(req *types.RegisterUsingTokenRequest) {
				req.Token = "nonexistent"
			},
			assert: require.Error,
		},
		{
			name: "wrong join method",
			modifyTokenRequest: func(req *types.RegisterUsingTokenRequest) {
				req.Token = "wrong-token"
			},
			assert: require.Error,
		},
		{
			name: "wrong host",
			modifyOracleRequest: func(req *proto.OracleSignedRequest) {
				req.Headers["Host"] = "somewhere.us-phoenix-1.example.com"
			},
			assert: require.Error,
		},
		{
			name: "invalid host region",
			modifyOracleRequest: func(req *proto.OracleSignedRequest) {
				req.Headers["Host"] = "auth.foo.oraclecloud.com"
			},
			assert: require.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenReq := &types.RegisterUsingTokenRequest{
				Token:        "my-token",
				HostID:       "test-node",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			}
			if tc.modifyTokenRequest != nil {
				tc.modifyTokenRequest(tokenReq)
			}
			_, err = a.registerUsingOracleMethod(
				ctx,
				tokenReq,
				func(challenge string) (*proto.OracleSignedRequest, error) {
					innerHeaders, outerHeaders, err := oracle.CreateSignedRequestWithProvider(provider, challenge)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					req := &proto.OracleSignedRequest{
						Headers:        mapFromHeader(outerHeaders),
						PayloadHeaders: mapFromHeader(innerHeaders),
					}

					if tc.modifyOracleRequest != nil {
						tc.modifyOracleRequest(req)
					}
					return req, nil
				},
				mockFetchClaims,
			)
			tc.assert(t, err)
		})
	}
}
