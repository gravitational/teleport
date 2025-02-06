{{- define "teleport.kube.agent.isUpgrade" -}}
{{- /* Checks if action is an upgrade from an old release that didn't support Secret storage */}}
{{- if .Release.IsUpgrade }}
  {{- $deployment := (lookup "apps/v1" "Deployment"  .Release.Namespace (include "teleport-kube-agent.fullname" .) ) -}}
  {{- if ($deployment) }}
true
  {{- else if .Values.unitTestUpgrade }}
true
  {{- end }}
{{- end }}
{{- end -}}
{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use teleport-kube-agent.fullname
*/}}
{{- define "teleport-kube-agent.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName (include "teleport-kube-agent.fullname" .) -}}
{{- end -}}

{{/*
Create the name of the service account to use for the post-delete hook
if serviceAccount is not defined or serviceAccount.name is empty, use "teleport-kube-agent.fullname"-delete-hook
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

{{/*
This is a modified version of the default fully qualified app name helper.
We diverge by defaulting to .Release.Name to be backwards compatible with older versions of this chart.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "teleport-kube-agent.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
