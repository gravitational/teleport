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

// Package testutils contains commonly used helpers for testing bot services.
//
// Note: we do not import the testing or testify packages to avoid accidentally
// bringing these dependencies into our production binaries.
package testutils

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const MockClusterName = "tele.blackmesa.gov"

// TestingT is a subset of *testing.T's interface. It is intentionally *NOT*
// compatible with testify's require and assert packages to avoid accidentally
// bringing those packages into production code. See: TestNotTestifyCompatible.
type TestingT interface {
	Cleanup(fn func())
	Context() context.Context
	Fatalf(format string, args ...any)
	Helper()
}

// MakeBot creates a bot server-side and returns the joining parameters.
func MakeBot(
	t TestingT,
	client *client.Client,
	name string,
	roles ...string,
) (*machineidv1pb.Bot, *onboarding.Config) {
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(t.Context(), &machineidv1pb.CreateBotRequest{
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
	if err != nil {
		t.Fatalf("failed to create bot: %v", err)
	}

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		t.Fatalf("failed to generate token name: %v", err)
	}

	tok, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(10*time.Minute),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: b.Metadata.Name,
		},
	)
	if err != nil {
		t.Fatalf("failed to build provision token: %v", err)
	}
	if err := client.CreateToken(t.Context(), tok); err != nil {
		t.Fatalf("failed to create provision token: %v", err)
	}

	return b, &onboarding.Config{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}
}

func SetWorkloadIdentityX509CAOverride(t TestingT, process *service.TeleportProcess) {
	t.Helper()

	const loadKeysFalse = false
	spiffeCA, err := process.GetAuthServer().GetCertAuthority(t.Context(), types.CertAuthID{
		DomainName: "root",
		Type:       types.SPIFFECA,
	}, loadKeysFalse)
	if err != nil {
		t.Fatalf("failed to get cert authority: %v", err)
	}

	spiffeCAX509KeyPairs := spiffeCA.GetTrustedTLSKeyPairs()
	if expected, got := 1, len(spiffeCAX509KeyPairs); expected != got {
		t.Fatalf("expected %s keypairs, got: %d", expected, got)
	}

	spiffeCACert, err := tlsca.ParseCertificatePEM(spiffeCAX509KeyPairs[0].Cert)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

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

func NewMockDiscoveredDB(t TestingT, name, discoveredName string) *types.DatabaseV3 {
	t.Helper()

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
		Labels: map[string]string{
			types.OriginLabel:         types.OriginCloud,
			types.DiscoveredNameLabel: discoveredName,
		},
	}, types.DatabaseSpecV3{
		Protocol: "mysql",
		URI:      "example.com:1234",
	})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	return db
}

func NewMockDiscoveredKubeCluster(t TestingT, name, discoveredName string) *types.KubernetesClusterV3 {
	t.Helper()

	kubeCluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: name,
			Labels: map[string]string{
				types.OriginLabel:         types.OriginCloud,
				types.DiscoveredNameLabel: discoveredName,
			},
		},
		types.KubernetesClusterSpecV3{},
	)
	if err != nil {
		t.Fatalf("failed to create kubernetes cluster: %v", err)
	}
	return kubeCluster
}

// FakeGetExecutablePath can be injected into outputs to ensure they output the
// same path in tests across multiple systems.
func FakeGetExecutablePath() (string, error) {
	return "/path/to/tbot", nil
}
