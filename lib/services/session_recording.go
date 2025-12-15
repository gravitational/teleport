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

package services

import (
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// ExtendWithSessionEnd extends the context with a session end event and
// rebuilds the resource from the event. An AccessChecker must be provided
// to allow access checks to other resources in the where clause.
func (ctx *Context) ExtendWithSessionEnd(sessionEnd apievents.AuditEvent, checker AccessChecker) {
	ctx.Session = sessionEnd
	ctx.Resource = rebuildResourceFromSessionEndEvent(sessionEnd)
	// AccessCheker is set here to allow access checks to other resources
	// in the where clause.
	ctx.AccessChecker = checker
}

// rebuildResourceFromSessionEndEvent rebuilds a resource from a session end event.
// This is used to reconstruct the resource that was active at the time of the session end event
// for audit log RBAC purposes.
func rebuildResourceFromSessionEndEvent(event apievents.AuditEvent) types.Resource {
	switch sEnd := event.(type) {
	case *apievents.SessionEnd:
		if sEnd == nil {
			return nil
		}
		switch sEnd.Protocol {
		case apievents.EventProtocolSSH:
			return &types.ServerV2{
				Kind:    types.KindNode,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      sEnd.ServerMetadata.ServerID,
					Namespace: sEnd.ServerMetadata.ServerNamespace,
					Labels:    sEnd.ServerMetadata.ServerLabels,
				},
				Spec: types.ServerSpecV2{
					Addr:     sEnd.ServerMetadata.ServerAddr,
					Hostname: sEnd.ServerMetadata.ServerHostname,
				},
			}
		case apievents.EventProtocolKube:
			return &types.KubernetesClusterV3{
				Kind:    types.KindKubernetesCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      sEnd.KubernetesClusterMetadata.KubernetesCluster,
					Namespace: apidefaults.Namespace,
					Labels:    sEnd.KubernetesClusterMetadata.KubernetesLabels,
				},
				Spec: types.KubernetesClusterSpecV3{},
			}
		}
	case *apievents.WindowsDesktopSessionEnd:
		if sEnd == nil {
			return nil
		}
		return &types.WindowsDesktopV3{
			ResourceHeader: types.ResourceHeader{
				Kind:    types.KindWindowsDesktop,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      sEnd.DesktopName,
					Namespace: apidefaults.Namespace,
					Labels:    sEnd.DesktopLabels,
				},
			},
			Spec: types.WindowsDesktopSpecV3{
				Addr:   sEnd.DesktopAddr,
				Domain: sEnd.Domain,
			},
		}
	case *apievents.DatabaseSessionEnd:
		if sEnd == nil {
			return nil
		}
		return &types.DatabaseV3{
			Kind:    types.KindDatabase,
			Version: types.V3,
			Metadata: types.Metadata{
				Name:      sEnd.DatabaseService,
				Namespace: apidefaults.Namespace,
				Labels:    sEnd.DatabaseLabels,
			},
			Spec: types.DatabaseSpecV3{
				Protocol: sEnd.DatabaseProtocol,
				URI:      sEnd.DatabaseURI,
			},
		}
	}
	return nil
}
