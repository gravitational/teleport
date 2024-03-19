{{/*
Expand the name of the chart.
*/}}
{{- define "teleport-cluster.operator.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
This is a modified version of the default fully qualified app name helper.
We diverge by always honouring "nameOverride" when it's set, as opposed to the
default behaviour of shortening if `nameOverride` is included in chart name.
This is done to avoid naming conflicts when including th chart in `teleport-cluster`
*/}}
{{- define "teleport-cluster.operator.fullname" -}}
    {{- if .Values.fullnameOverride }}
        {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
    {{- else }}
        {{- if .Values.nameOverride }}
            {{- printf "%s-%s" .Release.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
        {{- else }}
            {{- if contains .Chart.Name .Release.Name }}
                {{- .Release.Name | trunc 63 | trimSuffix "-" }}
            {{- else }}
                {{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
            {{- end }}
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
{{- $clusterAddr := include "teleport-cluster.auth.serviceFQDN" . -}}
{{- if empty $clusterAddr -}}
    {{- required "The `teleportAddress` value is mandatory when deploying a standalone operator." .Values.teleportAddress -}}
    {{- if and (eq .Values.joinMethod "kubernetes") (empty .Values.teleportClusterName) (not (hasSuffix ":3025" .Values.teleportAddress)) -}}
        {{- fail "When joining using the Kubernetes JWKS join method, you must set the value `teleportClusterName`" -}}
    {{- end -}}
{{- else -}}
    {{- $clusterAddr | printf "%s:3025" -}}
{{- end -}}
{{- end -}}

{{- /* This template is a placeholder.
If we are imported by the main chart "teleport-cluster" it is overridden*/ -}}
{{- define "teleport-cluster.auth.serviceFQDN" -}}{{- end }}
