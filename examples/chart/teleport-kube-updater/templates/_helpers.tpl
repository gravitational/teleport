{{- define "teleport-kube-updater.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "teleport-kube-updater.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "teleport-kube-updater.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "teleport-kube-updater.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- printf "%s-updater" (include "teleport-kube-updater.fullname" .) }}
{{- end }}
{{- end }}

{{- define "teleport-kube-updater.app.fullname" -}}
{{- .Release.Name -}}
{{- end -}}

{{- define "teleport-kube-updater.version" -}}
{{- if .Values.teleportVersionOverride -}}
  {{- .Values.teleportVersionOverride -}}
{{- else -}}
  {{- .Chart.Version -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-updater.baseImage" -}}
  {{- .Values.image -}}
{{- end -}}

{{- define "teleport-kube-updater.app.baseImage" -}}
  {{- .Values.appImage | required "agentImage must be provided" -}}
{{- end -}}

{{- define "teleport-kube-updater.app.containerName" -}}
  {{- .Values.containerName | required "containerName must be provided" -}}
{{- end -}}

{{ define "teleport-kube-updater.app.syncPeriod" -}}
{{ .Values.syncPeriod }}
{{- end }}

{{ define "teleport-kube-updater.proxyAddr" -}}
{{ .Values.proxyAddr | required "proxyAddr must be provided" }}
{{- end }}

{{- define "teleport-kube-updater.image" -}}
{{ include "teleport-kube-updater.baseImage" . }}:{{ include "teleport-kube-updater.version" . }}
{{- end -}}
