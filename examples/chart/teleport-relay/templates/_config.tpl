{{- define "teleport-relay.config" -}}
version: v3
teleport:
  join_params:
    method: {{ required "joinParams.method is required in chart values" .Values.joinParams.method | quote }}
    token_name: "/etc/teleport-secrets/auth-token"
  proxy_server: {{ required "proxyAddr is required in chart values" .Values.proxyAddr | quote }}
  log:
    severity: {{ required "log.level is required in chart values" .Values.log.level | quote }}
    format:
      output: {{ required "log.format is required in chart values" .Values.log.format | quote }}
  diag_addr: "0.0.0.0:3000"
  {{- with .Values.shutdownDelay }}
  shutdown_delay: {{ quote . }}
  {{- end }}
auth_service:
  enabled: false
proxy_service:
  enabled: false
ssh_service:
  enabled: false
relay_service:
  enabled: true
  relay_group: {{ required "relayGroup is required in chart values" .Values.relayGroup | quote }}
  {{- if le (int .Values.targetConnectionCount) 0 }}
    {{ fail "targetConnectionCount must be greater than 0" }}
  {{- end }}
  target_connection_count: {{ int .Values.targetConnectionCount }}
  {{- if empty .Values.publicHostnames }}
    {{- fail "publicHostnames cannot be empty in chart values" }}
  {{- end }}
  public_hostnames:
    {{- toYaml .Values.publicHostnames | nindent 4 }}
  transport_listen_addr: "0.0.0.0:3040"
  peer_listen_addr: "0.0.0.0:3041"
  tunnel_listen_addr: "0.0.0.0:3042"
  {{- if .Values.proxyProtocol }}
  transport_proxy_protocol: true
  tunnel_proxy_protocol: true
  {{- end }}
{{- end -}}
