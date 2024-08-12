{{- define "tbot.config" -}}
version: v2
{{- if .Values.teleportProxyAddress }}
proxy_server: {{ .Values.teleportProxyAddress }}
{{- end }}
{{- if .Values.teleportAuthAddress }}
auth_server: {{ .Values.teleportAuthAddress }}
{{- end }}
onboarding:
  join_method: {{ .Values.joinMethod }}
  token: {{ .Values.token }}
{{- if eq .Values.persistence "disabled" }}
storage:
  type: memory
{{- else if eq .Values.persistence "secret" }}
storage:
  type: kubernetes-secret
  name: {{ include "tbot.fullname" . }}
{{- end }}
{{- if or (.Values.defaultOutput.enabled) (.Values.outputs) }}
outputs:
{{- if .Values.defaultOutput.enabled }}
  - type: identity
    destination:
      type: kubernetes-secret
      name: {{ .Values.defaultOutput.secretName }}
{{- end }}
{{- if .Values.outputs }}
{{- toYaml .Values.outputs | nindent 6}}
{{- end }}
{{- end }}
{{- if .Values.services }}
services: {{- toYaml .Values.services | nindent 6}}
{{- end }}
{{- end -}}
