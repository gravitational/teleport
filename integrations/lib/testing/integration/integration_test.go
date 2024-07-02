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

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport/api"
)

type IntegrationSuite struct {
	BaseSetup
}

func TestIntegration(t *testing.T) { suite.Run(t, &IntegrationSuite{}) }

func (s *IntegrationSuite) SetupTest() {
	s.BaseSetup.SetupService()
}

func (s *IntegrationSuite) TestVersion() {
	t := s.T()

	versionMin, err := semver.NewVersion("12.0.0")
	require.NoError(t, err)
	versionMax, err := semver.NewVersion(api.Version)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, 1, s.Integration.Version().Compare(*versionMin))
	assert.LessOrEqual(t, 0, s.Integration.Version().Compare(*versionMax))
}
