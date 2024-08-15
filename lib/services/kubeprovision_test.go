/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/google/go-cmp/cmp"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"testing"
)

func TestKubeProvisionMarshalling(t *testing.T) {
	const kubeProvisionYAML = `---
kind: kube_provision
version: v1
metadata:
  name: test-provision
  labels:
    '*': '*'
spec:
  clusterRoles:
  - metadata:
      name: teleport-role1
    rules:
    - apiGroups:
      - ""
      resources:
      - pods
      verbs: ["get", "list"]
  - metadata:
      name: teleport-agg-role
    somefield:
      somefield2: "test"
    aggregationRule:
      clusterRoleSelectors:
      - matchLabels:
          rbac.authorization.k8s.io/aggregate-to-view: "true"
    rules:
    - apiGroups:
      - ""
      resources:
      - pods
      verbs: ["get", "watch"]
  clusterRoleBindings:
  - metadata:
      name: teleport-role1
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: teleport-role1
    subjects:
    - apiGroup: rbac.authorization.k8s.io
      kind: Group
      name: teleport-role1`

	data, err := utils.ToJSON([]byte(kubeProvisionYAML))
	require.NoError(t, err)

	unmarshalled, err := UnmarshalKubeProvision(data)
	require.NoError(t, err)

	kubeProvision := kubeprovisionv1.KubeProvision{
		Kind:    types.KindKubeProvision,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:   "test-provision",
			Labels: map[string]string{"*": "*"},
		},
		Spec: &kubeprovisionv1.KubeProvisionSpec{
			ClusterRoles: []*kubeprovisionv1.ClusterRole{
				{
					Metadata: &kubeprovisionv1.KubeObjectMeta{Name: "teleport-role1"},
					Rules: []*kubeprovisionv1.PolicyRule{
						{
							Resources: []string{"pods"},
							ApiGroups: []string{""},
							Verbs:     []string{"get", "list"},
						},
					},
				},
				{
					Metadata: &kubeprovisionv1.KubeObjectMeta{
						Name: "teleport-agg-role",
					},
					AggregationRule: &kubeprovisionv1.AggregationRule{
						ClusterRoleSelectors: []*kubeprovisionv1.LabelSelector{
							{
								MatchLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"},
							},
						},
					},
					Rules: []*kubeprovisionv1.PolicyRule{
						{
							Resources: []string{"pods"},
							ApiGroups: []string{""},
							Verbs:     []string{"get", "watch"},
						},
					},
				},
			},
			ClusterRoleBindings: []*kubeprovisionv1.ClusterRoleBinding{
				{
					Metadata: &kubeprovisionv1.KubeObjectMeta{Name: "teleport-role1"},
					Subjects: []*kubeprovisionv1.Subject{
						{
							Kind:     "Group",
							ApiGroup: "rbac.authorization.k8s.io",
							Name:     "teleport-role1",
						},
					},
					RoleRef: &kubeprovisionv1.RoleRef{
						Kind:     "ClusterRole",
						ApiGroup: "rbac.authorization.k8s.io",
						Name:     "teleport-role1",
					},
				},
			},
		},
	}
	require.Empty(t, compareKubeProvisions(unmarshalled, &kubeProvision))

	marshalledData, err := MarshalKubeProvision(unmarshalled)
	require.NoError(t, err)

	unmarshalled2, err := UnmarshalKubeProvision(marshalledData)
	require.NoError(t, err)
	require.Empty(t, compareKubeProvisions(unmarshalled, unmarshalled2))
}

func compareKubeProvisions(expected, actual *kubeprovisionv1.KubeProvision) string {
	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}
	return cmp.Diff(expected, actual, cmpOpts...)
}
