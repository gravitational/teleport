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

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegrationProxySuite struct {
	ProxySetup
}

func TestIntegrationProxy(t *testing.T) { suite.Run(t, &IntegrationProxySuite{}) }

func (s *IntegrationProxySuite) SetupTest() {
	s.ProxySetup.SetupService()
}

func (s *IntegrationProxySuite) TestPing() {
	t := s.T()

	var bootstrap Bootstrap
	user, err := bootstrap.AddUserWithRoles("vladimir", "editor")
	require.NoError(t, err)
	err = s.Integration.Bootstrap(s.Context(), s.Auth, bootstrap.Resources())
	require.NoError(t, err)

	identity, err := s.Integration.Sign(s.Context(), s.Auth, user.GetName())
	require.NoError(t, err)

	client, err := s.Integration.NewSignedClient(s.Context(), s.Proxy, identity, user.GetName())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	_, err = client.Ping(s.Context())
	require.NoError(t, err)
}
