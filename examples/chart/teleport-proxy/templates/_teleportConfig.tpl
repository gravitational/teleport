{{/*
  auth_server and join_params are treated special only because they're
  required.
*/}}
{{- define "teleport-proxy.generatedTeleportConfig" -}}
{{- $joinParams := mustDeepCopy .Values.join_params -}}
{{- $_ := set $joinParams "token_name" (include "teleport-proxy.token_name" .) -}}
teleport:
  auth_server: {{ include "teleport-proxy.auth_server" . | quote }}
  join_params:
{{- toYaml $joinParams | nindent 4 }}
{{- end -}}

{{- define "teleport-proxy.teleportConfig" -}}
{{- $generated := include "teleport-proxy.generatedTeleportConfig" . | fromYaml -}}
{{- $user := deepCopy (default dict .Values.teleportConfig) -}}
{{- $teleportConfig := mergeOverwrite $generated $user -}}
{{- toYaml $teleportConfig -}}
{{- end -}}
