/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockedAlertGetter struct {
	alerts []types.ClusterAlert
}

func (mag *mockedAlertGetter) GetClusterAlerts(ctx context.Context, query types.GetClusterAlertsRequest) ([]types.ClusterAlert, error) {
	return mag.alerts, nil
}

func mockAlertGetter(alerts []types.ClusterAlert) ClusterAlertGetter {
	return &mockedAlertGetter{
		alerts: alerts,
	}
}

func TestShowClusterAlerts(t *testing.T) {
	tests := map[string]struct {
		alerts  []types.ClusterAlert
		wantOut string
	}{
		"Single message": {
			alerts: []types.ClusterAlert{
				{
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_MEDIUM,
						Message:  "someMessage",
					},
				},
			},
			wantOut: "\x1b[33msomeMessage\x1b[0m\n\n",
		},
		"Sorted messages": {
			alerts: []types.ClusterAlert{
				{
					ResourceHeader: types.ResourceHeader{
						Metadata: types.Metadata{
							Labels: map[string]string{
								"someLabel": "yes",
							},
						},
					},
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_MEDIUM,
						Message:  "someOtherMessage",
					},
				}, {
					Spec: types.ClusterAlertSpec{
						Severity: types.AlertSeverity_HIGH,
						Message:  "someMessage",
					},
				},
			},

			wantOut: "\x1b[31msomeMessage\x1b[0m\n\n\x1b[33msomeOtherMessage\x1b[0m\n\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			alertGetter := mockAlertGetter(test.alerts)
			var got bytes.Buffer
			err := ShowClusterAlerts(context.Background(), alertGetter, &got, nil, types.AlertSeverity_LOW)
			require.NoError(t, err)
			require.Equal(t, test.wantOut, got.String())
		})
	}
}

func TestFormatLabels(t *testing.T) {
	basicLabels := map[string]string{
		"c": "d",
		"a": "b",
	}
	namespacedLabels := map[string]string{
		types.TeleportNamespace + "/foo":          "abc",
		types.TeleportInternalLabelPrefix + "bar": "def",
		types.TeleportHiddenLabelPrefix + "baz":   "ghi",
	}
	allLabels := make(map[string]string)
	maps.Copy(allLabels, basicLabels)
	maps.Copy(allLabels, namespacedLabels)
	tests := []struct {
		desc    string
		labels  map[string]string
		verbose bool
		want    string
	}{
		{
			desc:   "handles nil labels",
			labels: nil,
			want:   "",
		}, {
			desc:   "sorts labels",
			labels: basicLabels,
			want:   "a=b,c=d",
		}, {
			desc:   "excludes teleport namespace labels in non-verbose mode",
			labels: allLabels,
			want:   "a=b,c=d",
		}, {
			desc:   "returns empty string if all labels are excluded out",
			labels: namespacedLabels,
			want:   "",
		}, {
			desc:    "includes all labels in verbose mode",
			labels:  allLabels,
			verbose: true,
			want:    "a=b,c=d,teleport.dev/foo=abc,teleport.hidden/baz=ghi,teleport.internal/bar=def",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			got := FormatLabels(test.labels, test.verbose)
			require.Equal(t, test.want, got)
		})
	}
}

func TestFormatResourceAccessIDs(t *testing.T) {
	t.Parallel()

	const (
		ARN1 = "arn:aws:iam::123456789012:role/Role1"
		ARN2 = "arn:aws:iam::123456789012:role/Role2"
	)

	rids := []types.ResourceAccessID{
		{
			Id: types.ResourceID{
				Kind:        types.KindApp,
				Name:        "aws_console",
				ClusterName: "cluster",
			},
			Constraints: &types.ResourceConstraints{
				Version: types.V1,
				Details: &types.ResourceConstraints_AwsConsole{
					AwsConsole: &types.AWSConsoleResourceConstraints{
						RoleArns: []string{ARN1, ARN2},
					},
				},
			},
		},
		{
			Id: types.ResourceID{
				Kind:        types.KindNode,
				Name:        "ssh_server",
				ClusterName: "cluster",
			},
			Constraints: nil,
		},
	}

	t.Run("with aws_console constraints", func(t *testing.T) {
		t.Parallel()

		out, err := FormatResourceAccessIDs(rids)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("[\"/cluster/app/aws_console (role_arns=%s,%s)\",\"/cluster/node/ssh_server\"]", ARN1, ARN2), out)
	})

	t.Run("with empty aws_console constraints", func(t *testing.T) {
		t.Parallel()

		rid := types.ResourceAccessID{
			Id: rids[0].Id,
			Constraints: &types.ResourceConstraints{
				Version: types.V1,
				Details: &types.ResourceConstraints_AwsConsole{
					AwsConsole: &types.AWSConsoleResourceConstraints{
						RoleArns: []string{},
					},
				},
			},
		}

		out, err := FormatResourceAccessIDs([]types.ResourceAccessID{rid})
		require.NoError(t, err)
		require.Equal(t, "[\"/cluster/app/aws_console (role_arns=)\"]", out)
	})

	t.Run("with empty or nil list", func(t *testing.T) {
		t.Parallel()

		out, err := FormatResourceAccessIDs(nil)
		require.NoError(t, err)
		require.Empty(t, out)

		out, err = FormatResourceAccessIDs([]types.ResourceAccessID{})
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("with namespace resource with SubResourceName", func(t *testing.T) {
		t.Parallel()

		rid := types.ResourceAccessID{
			Id: types.ResourceID{
				ClusterName:     "cluster",
				Kind:            types.KindKubeNamespace,
				Name:            "my-kube-cluster",
				SubResourceName: "production",
			},
		}

		out, err := FormatResourceAccessIDs([]types.ResourceAccessID{rid})
		require.NoError(t, err)
		require.Equal(t, "[\"/cluster/namespace/my-kube-cluster/production\"]", out)
	})

	t.Run("with kube pod resource with SubResourceName", func(t *testing.T) {
		t.Parallel()

		rid := types.ResourceAccessID{
			Id: types.ResourceID{
				ClusterName:     "cluster",
				Kind:            types.AccessRequestPrefixKindKubeNamespaced + "pods",
				Name:            "my-kube-cluster",
				SubResourceName: "default/nginx",
			},
		}

		out, err := FormatResourceAccessIDs([]types.ResourceAccessID{rid})
		require.NoError(t, err)
		require.Equal(t, "[\"/cluster/kube:ns:pods/my-kube-cluster/default/nginx\"]", out)
	})
}
