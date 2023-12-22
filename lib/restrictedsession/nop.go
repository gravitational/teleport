//go:build !bpf || 386
// +build !bpf 386

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

package restrictedsession

import "github.com/gravitational/teleport/lib/service/servicecfg"

// New returns a new NOP service. Note this function does nothing.
func New(config *servicecfg.RestrictedSessionConfig, wc RestrictionsWatcherClient) (Manager, error) {
	return &NOP{}, nil
}
