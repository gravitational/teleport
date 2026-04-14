//go:build windows

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

package identityapi

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

func ServiceBuilder(cfg *Config, defaultCredentialLifetime bot.CredentialLifetime) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		return nil, trace.NotImplemented("identity-api service is not supported on Windows")
	}
	return bot.NewServiceBuilder(ServiceType, cfg.Name, buildFn)
}
