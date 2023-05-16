{{- define "teleport-cluster.proxy.config.scratch" -}}
ssh_service:
  enabled: false
auth_service:
  enabled: false
proxy_service:
  enabled: true
{{- end -}}

{{- define "teleport-cluster.proxy.config.custom" -}}
{{ fail "'custom' mode has been removed with chart v12 because of the proxy/auth split breaking change, see https://goteleport.com/docs/deploy-a-cluster/helm-deployments/migration-v12/" }}
{{- end -}}
