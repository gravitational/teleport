{{ if not .Values.teleportProxyAddress }}
  {{- $_ := required "`teleportProxyAddress` must be provided" "" }}
{{ end }}
{{ if not .Values.clusterName }}
  {{- $_ := required "`clusterName` must be provided" "" }}
{{ end }}
{{ if not .Values.token }}
  {{- $_ := required "`token` must be provided" "" }}
{{ end }}
{{ if and (not .Values.workloadIdentitySelector.name) (not .Values.workloadIdentitySelector.labels) }}
  {{- $_ := required "`workloadIdentitySelector.name` or `workloadIdentitySelector.labels` must be provided" "" }}
{{ end }}
{{ if and (.Values.workloadIdentitySelector.name) (.Values.workloadIdentitySelector.labels) }}
  {{- $_ := required "Either `workloadIdentitySelector.name` or `workloadIdentitySelector.labels` can be provided, not both" "" }}
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
  selector: {{ toYaml .Values.workloadIdentitySelector | nindent 4 }}
  attestors:
    kubernetes:
      enabled: true
      kubelet:
        skip_verify: true
{{- end -}}
