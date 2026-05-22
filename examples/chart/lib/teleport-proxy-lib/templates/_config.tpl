{{- define "teleport-proxy-lib.internal.config" }}
{{- $configTemplate := printf "teleport-proxy-lib.internal.config.%s" .Values.chartMode -}}
{{- if (contains ":" .Values.clusterName) -}}
  {{- fail "clusterName must not contain a colon, you can override the cluster's public address with publicAddr" -}}
{{- end -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-proxy
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.config }}
    {{- toYaml .Values.extraLabels.config | nindent 4 }}
    {{- end }}
{{- if .Values.annotations.config }}
  annotations: {{- toYaml .Values.annotations.config | nindent 4 }}
{{- end }}
data:
  teleport.yaml: |2
    {{- mustMergeOverwrite (include $configTemplate . | fromYaml) .Values.teleportConfig | toYaml | nindent 4 -}}
{{- end }}{{/* config */}}
