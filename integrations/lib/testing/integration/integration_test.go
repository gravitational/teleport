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

	"github.com/hashicorp/go-version"
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

	versionMin, err := version.NewVersion("v12.0.0")
	require.NoError(t, err)
	versionMax, err := version.NewVersion(api.Version)
	require.NoError(t, err)

	assert.True(t, s.Integration.Version().GreaterThanOrEqual(versionMin))
	assert.True(t, s.Integration.Version().LessThanOrEqual(versionMax))
}
