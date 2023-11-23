{{/*
Expand the name of the chart.
*/}}
{{- define "teleport-cluster.operator.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "teleport-cluster.operator.fullname" -}}
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
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-cluster.operator.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name (include "teleport-cluster.operator.fullname" .) -}}
{{- end -}}

{{- define "teleport-cluster.version" -}}
{{- coalesce .Values.teleportVersionOverride .Chart.Version }}
{{- end -}}

{{- define "teleport-cluster.majorVersion" -}}
{{- (semver (include "teleport-cluster.version" .)).Major -}}
{{- end -}}

{{/* Operator selector labels */}}
{{- define "teleport-cluster.operator.selectorLabels" -}}
app.kubernetes.io/name: '{{ include "teleport-cluster.operator.name" . }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'operator'
{{- end -}}

{{/* Operator all labels */}}
{{- define "teleport-cluster.operator.labels" -}}
{{ include "teleport-cluster.operator.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-cluster.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-cluster.majorVersion" . }}'
{{- end -}}

{{/* Teleport auth or proxy address */}}
{{- define "teleport-cluster.operator.teleportAddress" -}}
{{ coalesce (include "teleport-cluster.auth.serviceFQDN" . | printf "%s:3025") .Values.authServer }}
{{- end -}}

{{- /* This template is a placeholder.
If we are imported by the main chart "teleport-cluster" it is overridden*/ -}}
{{- define "teleport-cluster.auth.serviceFQDN" -}}{{- end }}
