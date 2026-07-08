{{/*
  auth_server and join_params are treated special only because they're
  required.
*/}}
{{- define "teleport-proxy.generatedTeleportConfig" -}}
teleport:
  auth_server: {{ include "teleport-proxy.authServer" . | quote }}
  join_params:
    method: {{ .Values.joinParams.method | quote }}
    token_name: {{ .Values.joinParams.tokenName | quote }}
{{- end -}}

{{- define "teleport-proxy.teleportConfig" -}}
{{- $generated := include "teleport-proxy.generatedTeleportConfig" . | fromYaml -}}
{{- $user := deepCopy (default dict .Values.teleportConfig) -}}
{{- $teleportConfig := mergeOverwrite $generated $user -}}
{{- toYaml $teleportConfig -}}
{{- end -}}
