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

package service

import (
	"context"

	"github.com/gravitational/teleport/lib/auth/join/boundkeypair"
	"github.com/gravitational/teleport/lib/auth/storage"
)

// boundKeypairStorageAdapter returns a storage interface implementation for
// bound keypair joining backed by the local agent storage.
func (p *TeleportProcess) boundKeypairStorageAdapter() boundkeypair.FS {
	return &boundKeypairAdapter{
		storage: p.storage,
	}
}

// boundKeypairAdapter satisfies the boundkeypair.FS interface and is suitable
// for managing persistence of bound keypair keys using agent local storage.
type boundKeypairAdapter struct {
	storage *storage.ProcessStorage
}

func (a *boundKeypairAdapter) Read(ctx context.Context, name string) ([]byte, error) {
	return a.storage.ReadBoundKeypairItem(ctx, name)
}

func (a *boundKeypairAdapter) Write(ctx context.Context, name string, value []byte) error {
	return a.storage.WriteBoundKeypairItem(ctx, name, value)
}
