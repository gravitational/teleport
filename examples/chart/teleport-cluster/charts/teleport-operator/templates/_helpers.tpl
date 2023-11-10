{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-cluster.operator.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}-operator
{{- end -}}

{{- define "teleport-cluster.version" -}}
{{- coalesce .Values.teleportVersionOverride .Chart.Version }}
{{- end -}}

{{- define "teleport-cluster.majorVersion" -}}
{{- (semver (include "teleport-cluster.version" .)).Major -}}
{{- end -}}

{{/* Operator selector labels */}}
{{- define "teleport-cluster.operator.selectorLabels" -}}
app.kubernetes.io/name: '{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}'
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
