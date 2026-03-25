{{- define "teleport-kube-updater.rolebinding" -}}
{{- if .rbac.create -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ .releaseName }}-updater
  namespace: {{ .releaseNamespace }}
{{- if .extraLabels.roleBinding }}
  labels:
  {{- toYaml .extraLabels.roleBinding | nindent 4 }}
{{- end }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ .releaseName }}-updater
subjects:
- kind: ServiceAccount
  name: {{ .serviceAccount.name }}
  namespace: {{ .releaseNamespace }}
{{- end -}}
{{- end -}}