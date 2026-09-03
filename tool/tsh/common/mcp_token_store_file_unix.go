//go:build !windows

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

package common

import "github.com/google/renameio/v2"

// Use a pending file directly because renameio.WriteFile preserves existing permissions.
func writeMCPOAuthCredentialsFile(path string, data []byte) error {
	pending, err := renameio.NewPendingFile(path, renameio.WithPermissions(0o600))
	if err != nil {
		return err
	}
	defer pending.Cleanup()
	if _, err := pending.Write(data); err != nil {
		return err
	}
	return pending.CloseAtomicallyReplace()
}
