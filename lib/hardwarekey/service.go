// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

// Package hardwarekey provides common hardware key types and functions.
// Hardware key types and functions which are not used within /api should be placed here.
package hardwarekey

import (
	"context"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/api/utils/keys/piv"
)

// NewService prepares a new hardware key service. If a running hardware key agent
// is found, this will return a hardware key agent service with a direct PIV service as backup.
// Otherwise, the direct PIV service will be returned.
func NewService(ctx context.Context, prompt hardwarekey.Prompt) hardwarekey.Service {
	hwks := piv.NewYubiKeyService(prompt)

	agentClient, err := NewAgentClient(ctx, DefaultAgentDir())
	if err == nil {
		return hardwarekeyagent.NewService(agentClient, hwks)
	}

	return hwks
}
