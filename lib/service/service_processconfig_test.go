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

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestGetServiceConfigYAML(t *testing.T) {
	t.Parallel()

	process := &TeleportProcess{
		Config: servicecfg.MakeDefaultConfig(),
	}

	t.Run("unknown subservice has no config", func(t *testing.T) {
		out, err := process.getServiceConfigYAML(context.Background(), "tls.config.generator")
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("auth subservice returns auth config", func(t *testing.T) {
		out, err := process.getServiceConfigYAML(context.Background(), "auth.expiry")
		require.NoError(t, err)
		require.Contains(t, out, "auth_service:")
	})
}
