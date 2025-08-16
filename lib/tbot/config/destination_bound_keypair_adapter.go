/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package config

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

// BoundKeypairBotFSAdapter is an adapter to use bot destinations with the FS
// abstraction for bound keypair joining. This allows keypair and state storage
// to be written to all supported bot destination types.
type BoundKeypairDestinationAdapter struct {
	destination bot.Destination
}

func (f *BoundKeypairDestinationAdapter) Read(ctx context.Context, name string) ([]byte, error) {
	bytes, err := f.destination.Read(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return bytes, nil
}

func (f *BoundKeypairDestinationAdapter) Write(ctx context.Context, name string, data []byte) error {
	return trace.Wrap(f.destination.Write(ctx, name, data))
}

// NewBoundkeypairDestinationAdapter creates a new destination adapter for bound
// keypair loading and storage.
func NewBoundkeypairDestinationAdapter(d bot.Destination) *BoundKeypairDestinationAdapter {
	return &BoundKeypairDestinationAdapter{
		destination: d,
	}
}
