/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package embeddedtbot

import (
	"testing"

	"github.com/stretchr/testify/require"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestBotKindComponents(t *testing.T) {
	t.Parallel()

	claimed := make(map[string]bot.Kind)
	for kind, component := range botKindComponents {
		other, ok := claimed[component]
		require.False(t, ok, "kinds %v and %v both report %q", kind, other, component)
		claimed[component] = kind
	}

	for value, name := range machineidv1.BotKind_name {
		kind := bot.Kind(value)
		if kind == bot.KindUnspecified {
			continue
		}
		require.Contains(t, botKindComponents, kind, "no component for %s", name)
	}
}
