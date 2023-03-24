{{- define "teleport-cluster.auth.config.scratch" -}}
proxy_service:
  enabled: false
ssh_service:
  enabled: false
auth_service:
  enabled: true
{{- end -}}

{{- define "teleport-cluster.auth.config.custom" -}}
{{ fail "'custom' mode has been removed with chart v12 because of the proxy/auth split breaking change, see https://goteleport.com/docs/deploy-a-cluster/helm-deployments/migration-v12/" }}
{{- end -}}
