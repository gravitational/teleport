//go:build !linux

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

package agent

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client/debug"
)

const (
	// BinaryName specifies the name of the updater binary.
	BinaryName = "teleport-update"
)

type invalidProcess struct {
	name string
}

func (i invalidProcess) Name() string {
	return i.name
}

func (i invalidProcess) Reload(ctx context.Context) error {
	return trace.NotImplemented("reload")
}

func (i invalidProcess) Sync(ctx context.Context) error {
	return trace.NotImplemented("sync")
}

func (i invalidProcess) IsEnabled(ctx context.Context) (bool, error) {
	return false, nil
}

func (i invalidProcess) IsActive(ctx context.Context) (bool, error) {
	return false, nil
}

func (i invalidProcess) IsPresent(ctx context.Context) (bool, error) {
	return false, nil
}

func targetServices(_ *Namespace) []ServiceFile {
	return []ServiceFile{}
}

func createTeleportProcess(ns *Namespace, debugClient *debug.Client, logger *slog.Logger) Process {
	return invalidProcess{
		name: ns.name,
	}
}

func createTbotProcess(ns *Namespace, debugClient *debug.Client, logger *slog.Logger) Process {
	return invalidProcess{
		name: ns.name,
	}
}

func (u *Updater) readMachineID() ([]byte, error) {
	return nil, trace.NotImplemented("readMachineID")
}
