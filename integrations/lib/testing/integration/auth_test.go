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

	"github.com/gravitational/teleport/api/types"
)

type IntegrationAuthSuite struct {
	AuthSetup
}

func TestIntegrationAuth(t *testing.T) { suite.Run(t, &IntegrationAuthSuite{}) }

func (s *IntegrationAuthSuite) SetupTest() {
	s.AuthSetup.SetupService()
}

func (s *IntegrationAuthSuite) TestBootstrap() {
	t := s.T()

	var bootstrap Bootstrap
	role, err := bootstrap.AddRole("foo", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = bootstrap.AddUserWithRoles("vladimir", role.GetName())
	require.NoError(t, err)
	err = s.Integration.Bootstrap(s.Context(), s.Auth, bootstrap.Resources())
	require.NoError(t, err)
}

func (s *IntegrationAuthSuite) TestPing() {
	t := s.T()

	var bootstrap Bootstrap
	user, err := bootstrap.AddUserWithRoles("vladimir", "editor")
	require.NoError(t, err)
	err = s.Integration.Bootstrap(s.Context(), s.Auth, bootstrap.Resources())
	require.NoError(t, err)

	client, err := s.Integration.NewClient(s.Context(), s.Auth, user.GetName())
	require.NoError(t, err)
	_, err = client.Ping(s.Context())
	require.NoError(t, err)
}
