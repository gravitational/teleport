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
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
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

// ConnectToDesktop establishes a desktop connection.
func (s *Handler) ConnectToDesktop(stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse]) error {
	msg, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	desktopURI := msg.GetTargetDesktop().GetDesktopUri()
	login := msg.GetTargetDesktop().GetLogin()
	if desktopURI == "" || login == "" {
		return trace.BadParameter("first message must contain a target desktop")
	}

	if msg.GetData() != nil {
		return trace.BadParameter("first message must not contain data")
	}

	parsed, err := uri.Parse(desktopURI)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.DaemonService.ConnectToDesktop(stream, parsed, login)
	return trace.Wrap(err)
}

// SetSharedDirectoryForDesktopSession opens a directory for a desktop session and enables file system operations for it.
// If there is no active desktop session associated with the specified desktop_uri and login,
// an error is returned.
func (s *Handler) SetSharedDirectoryForDesktopSession(ctx context.Context, in *api.SetSharedDirectoryForDesktopSessionRequest) (*api.SetSharedDirectoryForDesktopSessionResponse, error) {
	parsed, err := uri.Parse(in.GetDesktopUri())
	if err != nil {
		return &api.SetSharedDirectoryForDesktopSessionResponse{}, trace.Wrap(err)
	}

	err = s.DaemonService.SetSharedDirectoryForDesktopSession(ctx, parsed, in.GetLogin(), in.GetPath())
	return &api.SetSharedDirectoryForDesktopSessionResponse{}, trace.Wrap(err)
}
