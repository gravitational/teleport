{{- define "tbot.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName .Release.Name -}}
{{- end -}}

{{- define "tbot.selectorLabels" -}}
app.kubernetes.io/name: '{{ .Release.Name }}'
app.kubernetes.io/component: 'tbot'
{{- end -}}

{{- define "tbot.labels" -}}
{{ include "tbot.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
{{- end -}}