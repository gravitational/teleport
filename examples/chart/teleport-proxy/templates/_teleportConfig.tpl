{{/*
  auth_server and join_params are treated special only because they're
  required.
*/}}
{{- define "teleport-proxy.generatedTeleportConfig" -}}
version: v3
teleport:
  auth_server: {{ include "teleport-proxy.authServer" . | quote }}
  join_params:
    method: {{ .Values.joinParams.method | quote }}
    token_name: {{ .Values.joinParams.tokenName | quote }}
{{- $azureClientId := dig "joinParams" "azure" "clientId" false .Values.AsMap -}}
{{- if $azureClientId }}
    azure:
      client_id: {{ $azureClientId | quote }}
{{- end }}
{{- end -}}

{{- define "teleport-proxy.teleportConfig" -}}
{{- $generated := include "teleport-proxy.generatedTeleportConfig" . | fromYaml -}}
{{- $user := deepCopy (default dict .Values.teleportConfig) -}}
{{- $teleportConfig := mergeOverwrite $generated $user -}}
{{- toYaml $teleportConfig -}}
{{- end -}}
