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

package host

import (
	"context"
	"log/slog"
	"os/exec"
	"os/user"

	"github.com/gravitational/trace"
)

func MaybeSetCommandCredentialAsUser(ctx context.Context, cmd *exec.Cmd, requestUser *user.User, logger *slog.Logger) error {
	return trace.NotImplemented("windows not yet supported")
}
