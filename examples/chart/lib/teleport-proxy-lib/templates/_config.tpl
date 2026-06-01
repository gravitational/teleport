{{- define "teleport-proxy-lib.internal.config" }}
{{- $proxy := .Values -}}{{/* Minimizes diff for refactoring. Remove unneeded variable in next PR. */}}
{{- $configTemplate := printf "teleport-proxy-lib.internal.config.%s" $proxy.chartMode -}}
{{- if (contains ":" $proxy.clusterName) -}}
  {{- fail "clusterName must not contain a colon, you can override the cluster's public address with publicAddr" -}}
{{- end -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-proxy
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if $proxy.extraLabels.config }}
    {{- toYaml $proxy.extraLabels.config | nindent 4 }}
    {{- end }}
{{- if $proxy.annotations.config }}
  annotations: {{- toYaml $proxy.annotations.config | nindent 4 }}
{{- end }}
data:
  teleport.yaml: |2
    {{- mustMergeOverwrite (include $configTemplate . | fromYaml) $proxy.teleportConfig | toYaml | nindent 4 -}}
{{- end }}{{/* config */}}
