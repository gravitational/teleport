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

package bot_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
)

func TestBot_RejectsDoubleStart(t *testing.T) {
	b, err := bot.New(bot.Config{
		Connection: connection.Config{
			Address:            "localhost:3025",
			AddressKind:        connection.AddressKindProxy,
			StaticProxyAddress: true,
		},
	})
	require.NoError(t, err)

	ctx := context.Background()

	_ = b.OneShot(ctx)

	err = b.OneShot(ctx)
	require.ErrorContains(t, err, "already been started")

	err = b.Run(ctx)
	require.ErrorContains(t, err, "already been started")
}
