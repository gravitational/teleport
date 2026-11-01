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

//go:build !unix && !windows

package agent

import (
	"context"

	"github.com/gravitational/trace"
)

func (li *LocalInstaller) Link(ctx context.Context, rev Revision, pathDir string, force bool) (revert func(context.Context) bool, err error) {
	return func(ctx context.Context) bool {
		return false
	}, trace.NotImplemented("Link")
}

func (li *LocalInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	return func(ctx context.Context) bool {
		return false
	}, trace.NotImplemented("LinkSystem")
}

func (li *LocalInstaller) TryLink(ctx context.Context, rev Revision, pathDir string) error {
	return trace.NotImplemented("TryLink")
}

func (li *LocalInstaller) TryLinkSystem(ctx context.Context) error {
	return trace.NotImplemented("TryLinkSystem")
}

func (li *LocalInstaller) Unlink(ctx context.Context, rev Revision, pathDir string) error {
	return trace.NotImplemented("Unlink")
}

func (li *LocalInstaller) UnlinkSystem(ctx context.Context) error {
	return trace.NotImplemented("UnlinkSystem")
}

func (li *LocalInstaller) IsLinked(ctx context.Context, rev Revision, pathDir string) (bool, error) {
	return false, trace.NotImplemented("IsLinked")
}
