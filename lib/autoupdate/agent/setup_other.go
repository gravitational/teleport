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

//go:build !linux && !windows

package agent

import (
	"context"

	"github.com/gravitational/trace"
)

const (
	defaultPathDir = "/usr/local/bin"
)

func (ns *Namespace) HasCustomTbot(ctx context.Context) (bool, error) {
	return false, nil
}

func (ns *Namespace) WriteTeleportService(_ context.Context, pathDir string, rev Revision) error {
	return trace.NotImplemented("WriteTeleportService")
}

func (ns *Namespace) WriteTbotService(ctx context.Context, pathDir string, rev Revision) error {
	return trace.NotImplemented("WriteTbotService")
}

func (ns *Namespace) Setup(ctx context.Context, path string, rev Revision, f SetupFeatures) error {
	return trace.NotImplemented("Setup")
}

func (ns *Namespace) Teardown(ctx context.Context) error {
	return trace.NotImplemented("Teardown")
}
