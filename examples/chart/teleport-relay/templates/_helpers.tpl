{{- define "teleport-relay.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
This is a modified version of the default fully qualified app name helper.
We diverge by always honouring "nameOverride" when it's set, as opposed to the
default behaviour of shortening if `nameOverride` is included in chart name.
This is done to avoid naming conflicts when including the chart in other charts.
*/}}
{{- define "teleport-relay.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "teleport-relay.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "teleport-relay.selectorLabels" -}}
app.kubernetes.io/name: {{ include "teleport-relay.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "teleport-relay.labels" -}}
helm.sh/chart: {{ include "teleport-relay.chart" . }}
{{ include "teleport-relay.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "teleport-relay.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "teleport-relay.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- required "serviceAccount.name is required in chart values if serviceAccount.create is false" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "teleport-relay.version" -}}
{{ default .Chart.AppVersion .Values.teleportVersionOverride }}
{{- end }}

{{- define "teleport-relay.baseImage" -}}
{{- if .Values.enterprise }}
  {{- .Values.enterpriseImage }}
{{- else }}
  {{- .Values.image }}
{{- end }}
{{- end }}

{{- define "teleport-relay.image" -}}
{{ include "teleport-relay.baseImage" . }}:{{ include "teleport-relay.version" . }}
{{- end }}

{{- define "teleport-relay.joinTokenSecretName" -}}
{{- if .Values.joinTokenSecret.create }}
{{- default (include "teleport-relay.fullname" .) .Values.joinTokenSecret.name }}
{{- else }}
{{- required "joinTokenSecret.name is required in chart values if joinTokenSecret.create is false" .Values.joinTokenSecret.name }}
{{- end }}
{{- end }}
