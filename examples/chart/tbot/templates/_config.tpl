{{ if not (or .Values.teleportAuthAddress .Values.teleportProxyAddress) }}
  {{- $_ := required "`teleportAuthAddress` or `teleportProxyAddress` must be provided" "" }}
{{ end }}
{{ if not .Values.clusterName }}
  {{- $_ := required "`clusterName` must be provided" "" }}
{{ end }}
{{ if not .Values.token }}
  {{- $_ := required "`token` must be provided" "" }}
{{ end }}
{{ if (and .Values.teleportAuthAddress .Values.teleportProxyAddress) }}
  {{- $_ := required "`teleportAuthAddress` and `teleportProxyAddress` are mutually exclusive" "" }}
{{ end }}
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
  type: kubernetes_secret
  name: {{ include "tbot.fullname" . }}
{{- else }}
  {{- required "'persistence' must be 'secret' or 'disabled'" "" }}
{{- end }}
{{- if or (.Values.defaultOutput.enabled) (.Values.outputs) }}
outputs:
{{- if .Values.defaultOutput.enabled }}
  - type: identity
    destination:
      type: kubernetes_secret
      name: {{ include "tbot.defaultOutputName" . }}
{{- end }}
{{- if .Values.outputs }}
{{- toYaml .Values.outputs | nindent 2}}
{{- end }}
{{- end }}
{{- if .Values.services }}
services: {{- toYaml .Values.services | nindent 2}}
{{- end }}
{{- end -}}
