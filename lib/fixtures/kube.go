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

package fixtures

// KubeClusterRoleTemplate is a template for a Kubernetes ClusterRole
// that Teleport uses to access Kubernetes resources.
const KubeClusterRoleTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: teleport
rules:
- apiGroups:
  - ""
  resources:
  - users
  - groups
  - serviceaccounts
  verbs:
  - impersonate
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
- apiGroups:
  - "authorization.k8s.io"
  resources:
  - selfsubjectaccessreviews
  - selfsubjectrulesreviews
  verbs:
  - create
`

// KubeClusterRoleBindingTemplate is a template for a Kubernetes ClusterRoleBinding
// that Teleport uses to access Kubernetes resources.
const KubeClusterRoleBindingTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: teleport
subjects:
- kind: Group
  name: group_name
  apiGroup: rbac.authorization.k8s.io`

// KubePresetAccessClusterRoleBindingTemplate is a template for the kube-access preset role
// ClusterRoleBinding that maps to the Kubernetes 'edit' ClusterRole.
const KubePresetAccessClusterRoleBindingTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport-preset-kube-access
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: edit
subjects:
- kind: Group
  name: teleport:kube-access
  apiGroup: rbac.authorization.k8s.io`

// KubePresetEditorClusterRoleBindingTemplate is a template for the kube-editor preset role
// ClusterRoleBinding that maps to the Kubernetes 'cluster-admin' ClusterRole.
const KubePresetEditorClusterRoleBindingTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport-preset-kube-editor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: Group
  name: teleport:kube-editor
  apiGroup: rbac.authorization.k8s.io`

// KubePresetAuditorClusterRoleBindingTemplate is a template for the kube-auditor preset role
// ClusterRoleBinding that maps to the Kubernetes 'view' ClusterRole.
const KubePresetAuditorClusterRoleBindingTemplate = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: teleport-preset-kube-auditor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
- kind: Group
  name: teleport:kube-auditor
  apiGroup: rbac.authorization.k8s.io`
