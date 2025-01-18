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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/join/oracle"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/trace"
)

func TestCheckHeaders(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	defaultChallenge := "abcd1234"
	defaultAuthHeader := fmt.Sprintf(`Signature headers="%s %s"`, oracle.DateHeader, oracle.ChallengeHeader)

	t.Run("ok", func(t *testing.T) {
		headers := formatHeaderFromMap(map[string]string{
			"Authorization":        defaultAuthHeader,
			oracle.DateHeader:      oracle.FormatDateHeader(clock.Now()),
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
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now()),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "date not signed",
			headers: map[string]string{
				"Authorization":        fmt.Sprintf(`Signature headers="%s"`, oracle.ChallengeHeader),
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now()),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "challenge not signed",
			headers: map[string]string{
				"Authorization":        fmt.Sprintf(`Signature headers="%s"`, oracle.DateHeader),
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now()),
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
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now().Add(10 * time.Minute)),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "date too late",
			headers: map[string]string{
				"Authorization":        defaultAuthHeader,
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now().Add(-10 * time.Minute)),
				oracle.ChallengeHeader: defaultChallenge,
			},
		},
		{
			name: "missing challenge",
			headers: map[string]string{
				"Authorization":   defaultAuthHeader,
				oracle.DateHeader: oracle.FormatDateHeader(clock.Now()),
			},
		},
		{
			name: "challenge does not match",
			headers: map[string]string{
				"Authorization":        defaultAuthHeader,
				oracle.DateHeader:      oracle.FormatDateHeader(clock.Now()),
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

func claimsResponse(claimMap map[string]string) oracle.AuthenticateClientResult {
	claims := make([]oracle.Claim, 0, len(claimMap))
	for k, v := range claimMap {
		claims = append(claims, oracle.Claim{Key: k, Value: v})
	}
	return oracle.AuthenticateClientResult{
		Principal: oracle.Principal{
			Claims: claims,
		},
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

	oracleAPIServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(claimsResponse(map[string]string{
			oracle.TenancyClaim:     makeTenancyID("foo"),
			oracle.CompartmentClaim: makeCompartmentID("bar"),
			oracle.InstanceClaim:    makeInstanceID("us-phoenix-1", "baz"),
		}))
		if !assert.NoError(t, err) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(oracleAPIServer.Close)

	_, err = a.registerUsingOracleMethod(
		ctx,
		func(challenge string) (*proto.RegisterUsingOracleMethodRequest, error) {
			innerHeaders, outerHeaders, err := oracle.CreateSignedRequest(provider, challenge)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &proto.RegisterUsingOracleMethodRequest{
				RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
					Token:        "my-token",
					HostID:       "test-node",
					Role:         types.RoleNode,
					PublicSSHKey: sshPublicKey,
					PublicTLSKey: tlsPublicKey,
				},
				Headers:      mapFromHeader(outerHeaders),
				InnerHeaders: mapFromHeader(innerHeaders),
			}, nil
		},
		oracleAPIServer.URL,
	)
	require.NoError(t, err)
}
