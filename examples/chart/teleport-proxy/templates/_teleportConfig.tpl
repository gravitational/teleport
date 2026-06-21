{{/*
  auth_server and join_params are treated special only because they're
  required.
*/}}
{{- define "teleport-proxy.generatedTeleportConfig" -}}
teleport:
  auth_server: {{ include "teleport-proxy.auth_server" . | quote }}
  join_params:
    method: {{ .Values.join_params.method | quote }}
    token_name: {{ .Values.join_params.token_name | quote }}
{{- end -}}
{{- define "teleport-proxy.teleportConfig" -}}
{{- $generated := include "teleport-proxy.generatedTeleportConfig" . | fromYaml -}}
{{- $user := deepCopy (default dict .Values.teleportConfig) -}}
{{- $teleportConfig := mergeOverwrite $generated $user -}}
{{- toYaml $teleportConfig -}}
{{- end -}}
