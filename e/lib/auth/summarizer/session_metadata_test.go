package summarizer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
)

func TestBuildSessionMetadata(t *testing.T) {
	tests := []struct {
		name        string
		event       apievents.AuditEvent
		kind        types.SessionKind
		contains    []string
		notContains []string
		empty       bool
	}{
		{
			name: "SSH session with hostname and labels",
			event: &apievents.SessionEnd{
				ServerMetadata: apievents.ServerMetadata{
					ServerHostname: "prod-bastion-01",
					ServerLabels: map[string]string{
						"env":    "production",
						"team":   "platform",
						"region": "us-east-1",
					},
				},
			},
			kind: types.SSHSessionKind,
			contains: []string{
				"SESSION METADATA:",
				"Server hostname: prod-bastion-01",
				"Server labels: env=production, region=us-east-1, team=platform",
			},
		},
		{
			name: "Kube session with cluster, labels, and pod",
			event: &apievents.SessionEnd{
				KubernetesClusterMetadata: apievents.KubernetesClusterMetadata{
					KubernetesCluster: "prod-cluster",
					KubernetesLabels: map[string]string{
						"env":    "production",
						"region": "us-west-2",
					},
				},
				KubernetesPodMetadata: apievents.KubernetesPodMetadata{
					KubernetesPodName:      "api-server-abc123",
					KubernetesPodNamespace: "default",
				},
			},
			kind: types.KubernetesSessionKind,
			contains: []string{
				"SESSION METADATA:",
				"Kubernetes cluster: prod-cluster",
				"Kubernetes labels: env=production, region=us-west-2",
				"Pod name: api-server-abc123",
				"Pod namespace: default",
			},
		},
		{
			name:  "nil event",
			event: nil,
			kind:  types.SSHSessionKind,
			empty: true,
		},
		{
			name:  "non-SessionEnd event type",
			event: &apievents.DatabaseSessionEnd{},
			kind:  types.SSHSessionKind,
			empty: true,
		},
		{
			name:  "empty SessionEnd",
			event: &apievents.SessionEnd{},
			kind:  types.SSHSessionKind,
			empty: true,
		},
		{
			name: "SSH hostname only, no labels",
			event: &apievents.SessionEnd{
				ServerMetadata: apievents.ServerMetadata{
					ServerHostname: "dev-box",
				},
			},
			kind: types.SSHSessionKind,
			contains: []string{
				"Server hostname: dev-box",
			},
			notContains: []string{
				"Server labels:",
			},
		},
		{
			name: "labels are sorted alphabetically",
			event: &apievents.SessionEnd{
				ServerMetadata: apievents.ServerMetadata{
					ServerHostname: "host",
					ServerLabels: map[string]string{
						"z": "last",
						"a": "first",
						"m": "middle",
					},
				},
			},
			kind: types.SSHSessionKind,
			contains: []string{
				"a=first, m=middle, z=last",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSessionMetadata(tt.event, tt.kind)

			if tt.empty {
				require.Empty(t, result)

				return
			}

			for _, s := range tt.contains {
				require.Contains(t, result, s)
			}
			for _, s := range tt.notContains {
				require.NotContains(t, result, s)
			}
		})
	}
}
