{{- define "teleport-proxy-lib.internal.predeploy_config" }}
{{- if .Values.validateConfigOnDeploy }}
{{- $configTemplate := printf "teleport-proxy-lib.internal.config.%s" .Values.chartMode -}}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-proxy-test
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.config }}
    {{- toYaml .Values.extraLabels.config | nindent 4 }}
    {{- end }}
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "4"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
data:
  teleport.yaml: |2
    {{- mustMergeOverwrite (include $configTemplate . | fromYaml) .Values.teleportConfig | toYaml | nindent 4 -}}
{{- end }}
{{- end }}{{/* predeploy_config */}}
