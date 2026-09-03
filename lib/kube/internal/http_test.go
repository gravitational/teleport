// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package internal

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIdentityForwardingOmitsZeroFields(t *testing.T) {
	t.Parallel()

	identity := testKubernetesIdentity()

	ctx := authz.ContextWithUser(t.Context(), authz.WrapIdentity(identity))
	ctx = authz.ContextWithClientSrcAddr(ctx, &net.TCPAddr{
		IP:   net.ParseIP("192.0.2.1"),
		Port: 12345,
	})

	requireCompactIdentity := func(t *testing.T, identity tlsca.Identity, encoded string) {
		t.Helper()

		legacy, err := json.Marshal(identity)
		require.NoError(t, err)
		require.Less(t, len(encoded), len(legacy))
		require.NotContains(t, encoded, `"RouteToApp"`)
		require.NotContains(t, encoded, `"RouteToDatabase"`)
		require.NotContains(t, encoded, `"AssetTag"`)
		require.Contains(t, encoded, `"PrivateKeyPolicy":"none"`)

		var decoded tlsca.Identity
		require.NoError(t, json.Unmarshal([]byte(encoded), &decoded))
		decodedLegacy, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.JSONEq(t, string(legacy), string(decodedLegacy))
	}

	t.Run("headers", func(t *testing.T) {
		headers, err := IdentityForwardingHeaders(ctx, make(http.Header))
		require.NoError(t, err)

		requireCompactIdentity(t, identity, headers.Get(teleportImpersonateUserHeader))
	})

	t.Run("round tripper", func(t *testing.T) {
		transport := NewImpersonatorRoundTripper(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requireCompactIdentity(t, identity, req.Header.Get(teleportImpersonateUserHeader))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    req,
			}, nil
		}))

		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "https://kube.example.com", nil)
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	})
}

var benchmarkIdentityJSON []byte

func BenchmarkMarshalIdentity(b *testing.B) {
	identity := testKubernetesIdentity()

	benchmarks := []struct {
		name    string
		marshal func() ([]byte, error)
	}{
		{
			name: "legacy",
			marshal: func() ([]byte, error) {
				return json.Marshal(identity)
			},
		},
		{
			name: "compact",
			marshal: func() ([]byte, error) {
				return marshalIdentity(identity)
			},
		},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()

			var encoded []byte
			for b.Loop() {
				var err error
				encoded, err = benchmark.marshal()
				if err != nil {
					b.Fatal(err)
				}
			}

			benchmarkIdentityJSON = encoded
			b.ReportMetric(float64(len(encoded)), "header_bytes")
		})
	}
}

func testKubernetesIdentity() tlsca.Identity {
	return tlsca.Identity{
		Username:          "alice@example.com",
		Groups:            []string{"developer", "production-kubernetes"},
		Usage:             []string{"usage:kube"},
		KubernetesGroups:  []string{"developers", "system:authenticated"},
		KubernetesUsers:   []string{"alice@example.com"},
		Expires:           time.Date(2026, time.August, 27, 14, 5, 0, 0, time.UTC),
		RouteToCluster:    "root",
		KubernetesCluster: "production",
		Traits: wrappers.Traits{
			"email": {"alice@example.com"},
			"teams": {"platform"},
		},
		TeleportCluster:         "root",
		MFAVerified:             "e87f3c2e-71d8-4e64-b11f-6b6fe68f2e81",
		PreviousIdentityExpires: time.Date(2026, time.August, 27, 22, 0, 0, 0, time.UTC),
		ActiveRequests:          []string{"4f51a2b8-45d8-4f82-9b09-11589cda68f9"},
		PrivateKeyPolicy:        keys.PrivateKeyPolicyNone,
		DeviceExtensions: tlsca.DeviceExtensions{
			DeviceID: "9a1bc564-aaf6-4a10-bf35-0a93e7619e58",
		},
		UserType:          types.UserTypeSSO,
		OriginClusterName: "root",
	}
}
