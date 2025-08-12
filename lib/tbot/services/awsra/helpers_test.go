/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

type testYAMLCase[T any] struct {
	name string
	in   T
}

func testYAML[T any](t *testing.T, tests []testYAMLCase[T]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := new(bytes.Buffer)
			encoder := yaml.NewEncoder(b)
			encoder.SetIndent(2)
			require.NoError(t, encoder.Encode(&tt.in))

			if golden.ShouldSet() {
				golden.Set(t, b.Bytes())
			}
			require.Equal(
				t,
				string(golden.Get(t)),
				b.String(),
				"results of marshal did not match golden file, rerun tests with GOLDEN_UPDATE=1",
			)

			unmarshaled, err := internal.UnmarshalYAMLConfig[T](b)
			require.NoError(t, err)
			require.Equal(t, tt.in, *unmarshaled, "unmarshaling did not result in same object as input")
		})
	}
}

type checkAndSetDefaulter interface {
	CheckAndSetDefaults() error
}

type testCheckAndSetDefaultsCase[T checkAndSetDefaulter] struct {
	name string
	in   func() T

	// want specifies the desired state of the checkAndSetDefaulter after
	// check and set defaults has been run. If want is nil, the Output is
	// compared to its initial state.
	want    checkAndSetDefaulter
	wantErr string
}

func testCheckAndSetDefaults[T checkAndSetDefaulter](t *testing.T, tests []testCheckAndSetDefaultsCase[T]) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in()
			err := got.CheckAndSetDefaults()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			want := tt.want
			if want == nil {
				want = tt.in()
			}
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}
}

func setWorkloadIdentityX509CAOverride(ctx context.Context, t *testing.T, process *service.TeleportProcess) {
	const loadKeysFalse = false
	spiffeCA, err := process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
		DomainName: "root",
		Type:       types.SPIFFECA,
	}, loadKeysFalse)
	require.NoError(t, err)

	spiffeCAX509KeyPairs := spiffeCA.GetTrustedTLSKeyPairs()
	require.Len(t, spiffeCAX509KeyPairs, 1)
	spiffeCACert, err := tlsca.ParseCertificatePEM(spiffeCAX509KeyPairs[0].Cert)
	require.NoError(t, err)

	// this is a bit of a hack: by adding the self-signed CA certificate to the
	// override chain we distribute a nonempty chain that we can test for, but
	// all validations will continue working and it's technically not a broken
	// intermediate chain (just a bit of a useless one)

	// (this is an unsynced write but we know that nothing is issuing
	// certificates just yet)
	process.GetAuthServer().SetWorkloadIdentityX509CAOverrideGetter(&staticOverrideGetter{chain: [][]byte{spiffeCACert.Raw}})
}

type staticOverrideGetter struct {
	chain [][]byte
}

var _ services.WorkloadIdentityX509CAOverrideGetter = (*staticOverrideGetter)(nil)

// GetWorkloadIdentityX509CAOverride implements [services.WorkloadIdentityX509CAOverrideGetter].
func (m *staticOverrideGetter) GetWorkloadIdentityX509CAOverride(ctx context.Context, name string, ca *tlsca.CertAuthority) (*tlsca.CertAuthority, [][]byte, error) {
	return ca, m.chain, nil
}

// makeBot creates a server-side bot and returns joining parameters.
func makeBot(t *testing.T, client *authclient.Client, name string, roles ...string) (*onboarding.Config, *machineidv1pb.Bot) {
	ctx := context.TODO()
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: roles,
			},
		},
	})
	require.NoError(t, err)

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)
	tok, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(10*time.Minute),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: b.Metadata.Name,
		})
	require.NoError(t, err)
	err = client.CreateToken(ctx, tok)
	require.NoError(t, err)

	return &onboarding.Config{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}, b
}

func defaultTestServerOpts(log *slog.Logger) testenv.TestServerOptFunc {
	return func(o *testenv.TestServersOpts) error {
		testenv.WithClusterName("root")(o)
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Logger = log
			cfg.Proxy.PublicAddrs = []utils.NetAddr{
				{AddrNetwork: "tcp", Addr: net.JoinHostPort("localhost", strconv.Itoa(cfg.Proxy.WebAddr.Port(0)))},
			}
			cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{
				cfg.Proxy.ReverseTunnelListenAddr,
			}
		})(o)

		return nil
	}
}
