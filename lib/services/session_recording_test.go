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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestRebuildResourceFromSessionEndEvent(t *testing.T) {
	tests := []struct {
		name  string
		event apievents.AuditEvent
		want  types.Resource
	}{
		{
			name:  "nil event",
			event: nil,
			want:  nil,
		},
		{
			name:  "nil with type set session.end",
			event: (*apievents.SessionEnd)(nil),
			want:  nil,
		},
		{
			name:  "nil with type set windows.desktop.session.end",
			event: (*apievents.WindowsDesktopSessionEnd)(nil),
			want:  nil,
		},
		{
			name:  "nil with type set database.session.end",
			event: (*apievents.DatabaseSessionEnd)(nil),
			want:  nil,
		},
		{
			name: "non session.end event",
			event: &apievents.UserLogin{
				Metadata: apievents.Metadata{
					ClusterName: "test-cluster",
				},
				UserMetadata: apievents.UserMetadata{
					User: "test-user",
				},
			},
			want: nil,
		},
		{
			name: "session.end with SSH session",
			event: &apievents.SessionEnd{
				Metadata: apievents.Metadata{
					ClusterName: "test-cluster",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					Protocol: apievents.EventProtocolSSH,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerID:        "server-id-123",
					ServerHostname:  "server-host-123",
					ServerAddr:      "server-addr-123",
					ServerNamespace: "default",
					ServerLabels: map[string]string{
						"env":  "prod",
						"team": "devops",
					},
				},
			},
			want: &types.ServerV2{
				Kind:    types.KindNode,
				Version: types.V2,
				Metadata: types.Metadata{
					Name:      "server-id-123",
					Namespace: "default",
					Labels: map[string]string{
						"env":  "prod",
						"team": "devops",
					},
				},
				Spec: types.ServerSpecV2{
					Addr:     "server-addr-123",
					Hostname: "server-host-123",
				},
			},
		},
		{
			name: "session.end for Kube",
			event: &apievents.SessionEnd{
				Metadata: apievents.Metadata{
					ClusterName: "test-cluster",
				},
				ConnectionMetadata: apievents.ConnectionMetadata{
					Protocol: apievents.EventProtocolKube,
				},
				KubernetesClusterMetadata: apievents.KubernetesClusterMetadata{
					KubernetesCluster: "kube-cluster-123",
					KubernetesLabels: map[string]string{
						"env":  "staging",
						"team": "backend",
					},
				},
			},
			want: &types.KubernetesClusterV3{
				Kind:    types.KindKubernetesCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "kube-cluster-123",
					Namespace: "default",
					Labels: map[string]string{
						"env":  "staging",
						"team": "backend",
					},
				},
				Spec: types.KubernetesClusterSpecV3{},
			},
		},
		{
			name: "windows sessio end",
			event: &apievents.WindowsDesktopSessionEnd{
				WindowsDesktopService: "win-desktop-service",
				DesktopName:           "win-desktop-name",
				DesktopAddr:           "win-desktop-addr",
				DesktopLabels: map[string]string{
					"env":  "prod",
					"team": "devops",
				},
			},
			want: &types.WindowsDesktopV3{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindWindowsDesktop,
					Version: types.V3,
					Metadata: types.Metadata{
						Name:      "win-desktop-name",
						Namespace: "default",
						Labels: map[string]string{
							"env":  "prod",
							"team": "devops",
						},
					},
				},
				Spec: types.WindowsDesktopSpecV3{
					Addr: "win-desktop-addr",
				},
			},
		},
		{
			name: "database session end",
			event: &apievents.DatabaseSessionEnd{
				DatabaseMetadata: apievents.DatabaseMetadata{
					DatabaseService: "db-service",
					DatabaseName:    "db-name",
					DatabaseLabels: map[string]string{
						"env":  "qa",
						"team": "analytics",
					},
				},
			},
			want: &types.DatabaseV3{
				Kind:    types.KindDatabase,
				Version: types.V3,
				Metadata: types.Metadata{
					Name:      "db-service",
					Namespace: "default",
					Labels: map[string]string{
						"env":  "qa",
						"team": "analytics",
					},
				},
				Spec: types.DatabaseSpecV3{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rebuildResourceFromSessionEndEvent(tt.event)
			require.Equal(t, tt.want, got)

			if tt.want == nil {
				return
			}

			sctx := &Context{}
			checker := &accessChecker{}
			sctx.ExtendWithSessionEnd(tt.event, checker)
			require.Equal(t, tt.want, sctx.Resource)
			require.Same(t, checker, sctx.AccessChecker)
			require.Same(t, tt.event, sctx.Session)
		})
	}
}
