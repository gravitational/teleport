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
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/utils"
)

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
