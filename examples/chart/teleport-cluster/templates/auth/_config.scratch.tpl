{{- define "teleport-cluster.auth.config.scratch" -}}
{{- required "'auth.teleportConfig' is required in scratch mode" .Values.auth.teleportConfig }}
proxy_service:
  enabled: false
ssh_service:
  enabled: false
auth_service:
  enabled: true
{{- end -}}

{{- define "teleport-cluster.auth.config.custom" -}}
{{ fail "'custom' mode has been depreacted with chart v12 because of the proxy/auth split, see http://link" }}
{{- end -}}
