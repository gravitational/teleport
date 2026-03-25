{{- define "teleport-kube-updater.role" -}}
{{- if .rbac.create -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ .releaseName }}-updater
  namespace: {{ .releaseNamespace }}
{{- if .extraLabels.role }}
  labels: {{- toYaml .extraLabels.role | nindent 4 }}
{{- end }}
rules:
# the updater needs to list pods to check their health
# it also needs to delete pods to unstuck Statefulset rollouts
- apiGroups:
    - ""
  resources:
    - pods
  verbs:
    - get
    - watch
    - list
    - delete
- apiGroups:
    - ""
  resources:
    - pods/status
  verbs:
    - get
    - watch
    - list
# the updater needs to get the secret created by the agent containing the
# maintenance window
- apiGroups:
    - ""
  resources:
    - secrets
  verbs:
    - watch
    - list
- apiGroups:
    - ""
  resources:
    - secrets
  verbs:
    - get
  resourceNames:
    - {{ .releaseName }}-shared-state
- apiGroups:
    - ""
  resources:
    - events
  verbs:
    - create
    - patch
# the updater needs to create, get, and update the configmap containing updater state
- apiGroups:
    - ""
  resources:
    - configmaps
  verbs:
    - create
    - watch
    - list
- apiGroups:
    - ""
  resources:
    - configmaps
  verbs:
    - get
    - update
  resourceNames:
    - {{ .releaseName }}-updater
# the controller in the updater must be able to watch deployments and
# statefulsets and get the one it should reconcile
- apiGroups:
    - "apps"
  resources:
    - deployments
    - statefulsets
    - deployments/status
    - statefulsets/status
  verbs:
    - get
    - watch
    - list
# However the updater should only update the agent it is watching
- apiGroups:
    - "apps"
  resources:
    - deployments
    - statefulsets
  verbs:
    - update
  resourceNames:
    - {{ .releaseName }}
- apiGroups:
    - coordination.k8s.io
  resources:
    - leases
  verbs:
    - create
- apiGroups:
    - coordination.k8s.io
  resourceNames:
    - {{ .releaseName }}
  resources:
    - leases
  verbs:
    - get
    - update
{{- end -}}
{{- end -}}