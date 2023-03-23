/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
