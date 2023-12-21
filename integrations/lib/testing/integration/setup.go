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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resources, err := s.Integration.tctl(s.Auth).GetAll(s.Context(), "nodes")
		assert.NoError(t, err)
		assert.NotEmpty(t, resources)
	}, 5*time.Second, time.Second)
}
