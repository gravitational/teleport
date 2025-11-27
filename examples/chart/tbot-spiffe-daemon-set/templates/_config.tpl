{{ if not .Values.teleportProxyAddress }}
  {{- $_ := required "`teleportProxyAddress` must be provided" "" }}
{{ end }}
{{ if not .Values.clusterName }}
  {{- $_ := required "`clusterName` must be provided" "" }}
{{ end }}
{{ if not .Values.token }}
  {{- $_ := required "`token` must be provided" "" }}
{{ end }}
{{- define "tbot-spiffe-daemon-set.config" -}}
version: v2
{{- if .Values.teleportProxyAddress }}
proxy_server: {{ .Values.teleportProxyAddress }}
{{- end }}
onboarding:
  join_method: {{ .Values.joinMethod }}
  token: {{ .Values.token }}
storage:
  type: memory
services:
- type: workload-identity-api
  listen: unix:///run/tbot/sockets/workload.sock
  selector:
    name: k8s-ds-example
  attestor:
    kubernetes:
      enabled: true
      kubelet:
        skip_verify: true
{{- end -}}
