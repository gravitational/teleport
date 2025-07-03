{{- define "teleport.kube.agent.isUpgrade" -}}
{{- /* Checks if action is an upgrade from an old release that didn't support Secret storage */}}
{{- if .Release.IsUpgrade }}
  {{- $deployment := (lookup "apps/v1" "Deployment"  .Release.Namespace .Release.Name ) -}}
  {{- if ($deployment) }}
true
  {{- else if .Values.unitTestUpgrade }}
true
  {{- end }}
{{- end }}
{{- end -}}
{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-kube-agent.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName .Release.Name -}}
{{- end -}}

{{/*
Create the name of the service account to use for the post-delete hook
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name-delete-hook
*/}}
{{- define "teleport-kube-agent.deleteHookServiceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- printf "%s-delete-hook" (include "teleport-kube-agent.serviceAccountName" . ) -}}
{{- else -}}
{{- (include "teleport-kube-agent.serviceAccountName" . ) -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-agent.version" -}}
{{- if .Values.teleportVersionOverride -}}
  {{- .Values.teleportVersionOverride -}}
{{- else -}}
  {{- .Chart.Version -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-agent.baseImage" -}}
{{- if .Values.enterprise -}}
  {{- .Values.enterpriseImage -}}
{{- else -}}
  {{- .Values.image -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-agent.image" -}}
{{ include "teleport-kube-agent.baseImage" . }}:{{ include "teleport-kube-agent.version" . }}
{{- end -}}

{{- define "teleport-kube-agent.rbac-role-name" -}}
{{- coalesce .Values.rbac.roleName .Values.roleName .Release.Name -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-rolebinding-name" -}}
{{- coalesce .Values.rbac.roleBindingName .Values.roleBindingName .Release.Name -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-clusterrole-name" -}}
{{- coalesce .Values.rbac.clusterRoleName .Values.rbac.clusterRoleName .Release.Name -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-clusterrolebinding-name" -}}
{{- coalesce .Values.rbac.clusterRoleBindingName .Values.rbac.clusterRoleBindingName .Release.Name -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-admin-clusterrole-name" -}}
{{- if .Values.rbac.adminClusterRoleName -}}
{{- .Values.rbac.adminClusterRoleName -}}
{{- else -}}
{{- printf "%s-cluster-admin" .Release.Name -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-admin-clusterrolebinding-name" -}}
{{- if .Values.rbac.adminClusterRoleBindingName -}}
{{- .Values.rbac.adminClusterRoleBindingName -}}
{{- else -}}
{{- printf "%s-cluster-admin" .Release.Name -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-agent.rbac-admin-group-name" -}}
{{- if .Values.rbac.adminGroupName -}}
{{- .Values.rbac.adminGroupName -}}
{{- else -}}
{{- printf "%s-cluster-admin" .Release.Name -}}
{{- end -}}
{{- end -}}
