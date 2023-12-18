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

package reversetunnel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectedProxyGetter(t *testing.T) {
	proxies := NewConnectedProxyGetter()

	var expectIDs []string
	ids := proxies.GetProxyIDs()
	require.Nil(t, ids)

	expectIDs = []string{}
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Nil(t, ids)

	expectIDs = []string{"test1", "test2"}
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = nil
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Nil(t, ids)
}
