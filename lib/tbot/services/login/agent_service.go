// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package login

import (
	"context"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/trace"
)

// AgentServiceBuilder returns a builder for the login agent service.
func AgentServiceBuilder(cfg *AgentConfig, opts ...AgentOpt) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}
		return &AgentService{
			cfg: cfg,
		}, nil
	}
	return bot.NewServiceBuilder(
		AgentServiceType,
		cfg.GetName(),
		buildFn,
	)
}

// AgentService implements a "login agent" for tsh to non-interactively bootstrap
// its identity from tbot.
type AgentService struct {
	cfg                       *AgentConfig
	defaultCredentialLifetime bot.CredentialLifetime
}

// Run the service until the given context is cancelled.
func (s *AgentService) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// String satisfies fmt.Stringer.
func (s *AgentService) String() string { return s.cfg.GetName() }
