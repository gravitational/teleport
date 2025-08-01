{{/*
Expand the name of the chart.
*/}}
{{- define "jira.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "jira.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "jira.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "jira.labels" -}}
helm.sh/chart: {{ include "jira.chart" . }}
{{ include "jira.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "jira.selectorLabels" -}}
app.kubernetes.io/name: {{ include "jira.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "jira.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "jira.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Use tbot-managed identity secret if tbot is enabled
*/}}
{{- define "jira.identitySecretName" -}}
{{- if .Values.teleport.identitySecretName -}}
{{- .Values.teleport.identitySecretName -}}
{{- else if .Values.tbot.enabled -}}
  {{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}-out
{{- end }}
{{- end -}}

{{- define "jira.identitySecretPath" -}}
{{- if .Values.tbot.enabled -}}
identity
{{- else -}}
{{- .Values.teleport.identitySecretPath -}}
{{- end -}}
{{- end -}}
