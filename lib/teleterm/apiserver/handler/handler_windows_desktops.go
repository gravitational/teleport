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

package handler

import (
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/ui"
)

func newAPIWindowsDesktop(clusterDesktop clusters.WindowsDesktop) *api.WindowsDesktop {
	desktop := clusterDesktop.WindowsDesktop
	apiLabels := makeAPILabels(ui.MakeLabelsWithoutInternalPrefixes(desktop.GetAllLabels()))

	return &api.WindowsDesktop{
		Uri:    clusterDesktop.URI.String(),
		Name:   desktop.GetName(),
		Addr:   desktop.GetAddr(),
		Logins: clusterDesktop.Logins,
		Labels: apiLabels,
	}
}
