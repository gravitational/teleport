{{- define "teleport-cluster.proxy.config.scratch" -}}
{{- required "'proxy.teleportConfig' is required in scratch mode" .Values.proxy.teleportConfig }}
ssh_service:
  enabled: false
auth_service:
  enabled: true
proxy_service:
  enabled: true
{{- end -}}

{{- define "teleport-cluster.proxy.config.custom" -}}
{{ fail "'custom' mode has been depreacted with chart v12 because of the proxy/auth split, see http://link" }}
{{- end -}}