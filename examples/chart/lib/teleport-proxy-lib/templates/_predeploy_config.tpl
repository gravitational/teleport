{{- define "teleport-proxy-lib.internal.predeploy_config" }}
{{- $proxy := .Values -}}{{/* Minimizes diff for refactoring. Remove unneeded variable in next PR. */}}
{{- if $proxy.validateConfigOnDeploy }}
{{- $configTemplate := printf "teleport-proxy-lib.internal.config.%s" $proxy.chartMode -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-proxy-test
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if $proxy.extraLabels.config }}
    {{- toYaml $proxy.extraLabels.config | nindent 4 }}
    {{- end }}
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "4"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
data:
  teleport.yaml: |2
    {{- mustMergeOverwrite (include $configTemplate . | fromYaml) $proxy.teleportConfig | toYaml | nindent 4 -}}
{{- end }}
{{- end }}{{/* predeploy_config */}}
