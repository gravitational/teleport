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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

type BaseSetup struct {
	Suite
	Integration *Integration
}

type AuthSetup struct {
	BaseSetup
	Auth         *AuthService
	CacheEnabled bool
}

type ProxySetup struct {
	AuthSetup
	Proxy *ProxyService
}

type SSHSetup struct {
	ProxySetup
	SSH *SSHService
}

func (s *BaseSetup) SetupSuite(t *testing.T) {
	logger.Init()
	err := logger.Setup(logger.Config{Severity: "debug"})
	require.NoError(t, err)
}

func (s *BaseSetup) SetupService() {
	t := s.T()
	var err error

	// We set such a big timeout because integration.NewFromEnv could start
	// downloading a Teleport *-bin.tar.gz file which can take a long time.
	ctx := s.SetContextTimeout(5 * time.Minute)
	integration, err := NewFromEnv(ctx)
	require.NoError(t, err)
	t.Cleanup(integration.Close)
	s.Integration = integration
}

func (s *AuthSetup) SetupSuite(t *testing.T) {
	s.CacheEnabled = false
	s.BaseSetup.SetupSuite(t)
}

func (s *AuthSetup) SetupService(authServiceOptions ...AuthServiceOption) {
	s.BaseSetup.SetupService()
	t := s.T()
	auth, err := s.Integration.NewAuthService(authServiceOptions...)
	require.NoError(t, err)
	s.StartApp(auth)
	s.Auth = auth

	ready, err := s.Auth.WaitReady(s.Context())
	require.NoError(t, err)
	require.True(t, ready, "auth is not ready")

	// Set CA Pin so that Proxy and SSH can register to auth securely.
	err = s.Integration.SetCAPin(s.Context(), s.Auth)
	require.NoError(t, err)
}

func (s *ProxySetup) SetupSuite(t *testing.T) {
	s.AuthSetup.SetupSuite(t)
}

func (s *ProxySetup) SetupService() {
	s.AuthSetup.SetupService()
	t := s.T()
	proxy, err := s.Integration.NewProxyService(s.Auth)
	require.NoError(t, err)
	s.StartApp(proxy)
	s.Proxy = proxy
	ready, err := s.Proxy.WaitReady(s.Context())
	require.NoError(t, err)
	require.True(t, ready, "proxy is not ready")
}

func (s *SSHSetup) SetupSuite(t *testing.T) {
	s.ProxySetup.SetupSuite(t)
}

func (s *SSHSetup) SetupService() {
	s.ProxySetup.SetupService()
	t := s.T()
	ssh, err := s.Integration.NewSSHService(s.Auth)
	require.NoError(t, err)
	s.StartApp(ssh)
	s.SSH = ssh
	ready, err := s.SSH.WaitReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready, "ssh is not ready")

	// Wait for node to show up on the server.
	require.Eventually(t, func() bool {
		resources, err := s.Integration.tctl(s.Auth).GetAll(s.Context(), "nodes")
		require.NoError(t, err)

		return len(resources) != 0
	}, 5*time.Second, time.Second)
}
