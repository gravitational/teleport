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

// Note: this lives in tbot to avoid import cycles since this depends on the
// config/identity/destinations packages.

package tbot

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

func TestLoadEmptyIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	dest := config.DestinationDirectory{
		Path: dir,
	}
	require.NoError(t, dest.CheckAndSetDefaults())

	_, err := identity.LoadIdentity(ctx, &dest, identity.BotKinds()...)
	require.Error(t, err)

	require.True(t, trace.IsNotFound(err))
}
