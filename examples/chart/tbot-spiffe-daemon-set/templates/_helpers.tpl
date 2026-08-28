{{- define "tbot-spiffe-daemon-set.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name (include "tbot-spiffe-daemon-set.fullname" .) -}}
{{- end -}}

{{- define "tbot-spiffe-daemon-set.selectorLabels" -}}
app.kubernetes.io/name: '{{ include "tbot-spiffe-daemon-set.name" . }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'tbot-spiffe-daemon-set'
{{- end -}}

{{- define "tbot-spiffe-daemon-set.labels" -}}
{{ include "tbot-spiffe-daemon-set.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "tbot-spiffe-daemon-set.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
This is a modified version of the default fully qualified app name helper.
We diverge by always honouring "nameOverride" when it's set, as opposed to the
default behaviour of shortening if `nameOverride` is included in chart name.
This is done to avoid naming conflicts when including the chart in other charts.
*/}}
{{- define "tbot-spiffe-daemon-set.fullname" -}}
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

{{- define "tbot-spiffe-daemon-set.version" -}}
{{- if .Values.teleportVersionOverride -}}
  {{- .Values.teleportVersionOverride -}}
{{- else -}}
  {{- .Chart.Version -}}
{{- end -}}
{{- end -}}

{{- define "tbot-spiffe-daemon-set.defaultOutputName" -}}
{{- include "tbot-spiffe-daemon-set.fullname" . }}-out
{{- end -}}
