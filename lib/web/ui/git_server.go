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

package ui

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/ui"
)

// GitServer describes a GitServer for webapp
type GitServer struct {
	// Kind is the kind of resource.
	Kind string `json:"kind"`
	// SubKind is a git server subkind such as GitHub
	SubKind string `json:"subKind"`
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
	// Hostname is this server hostname
	Hostname string `json:"hostname"`
	// Addr is this server ip address
	Addr string `json:"addr"`
	// Labels is this server list of labels
	Labels []ui.Label `json:"tags"`
	// RequireRequest indicates if a returned resource is only accessible after an access request
	RequiresRequest bool `json:"requiresRequest,omitempty"`
	// GitHub contains metadata for GitHub proxy severs.
	GitHub *GitHubServerMetadata `json:"github,omitempty"`
}

// GitHubServerMetadata contains metadata for GitHub proxy severs.
type GitHubServerMetadata struct {
	Integration  string `json:"integration"`
	Organization string `json:"organization"`
}

// MakeGitServer creates a git server object for the web ui
func MakeGitServer(clusterName string, server types.Server, requiresRequest bool) GitServer {
	serverLabels := server.GetStaticLabels()
	serverCmdLabels := server.GetCmdLabels()
	uiLabels := ui.MakeLabelsWithoutInternalPrefixes(serverLabels, ui.TransformCommandLabels(serverCmdLabels))

	uiServer := GitServer{
		Kind:            server.GetKind(),
		ClusterName:     clusterName,
		Labels:          uiLabels,
		Name:            server.GetName(),
		Hostname:        server.GetHostname(),
		Addr:            server.GetAddr(),
		SubKind:         server.GetSubKind(),
		RequiresRequest: requiresRequest,
	}

	if server.GetSubKind() == types.SubKindGitHub {
		if github := server.GetGitHub(); github != nil {
			uiServer.GitHub = &GitHubServerMetadata{
				Integration:  github.Integration,
				Organization: github.Organization,
			}
		}
	}
	return uiServer
}
