{{- define "teleport-cluster.proxy.config.common" -}}
{{- $logLevel := (coalesce .Values.logLevel .Values.log.level "INFO") -}}
version: v3
teleport:
  join_params:
    method: kubernetes
    token_name: "{{.Release.Name}}-proxy"
  auth_server: "{{ include "teleport-cluster.auth.serviceFQDN" . }}:3025"
  log:
    severity: {{ $logLevel }}
    output: {{ .Values.log.output }}
    {{- if .Values.log.watch_log_file }}
    watch_log_file: {{ .Values.log.watch_log_file }}
    {{- end }}
    format:
      output: {{ .Values.log.format }}
      extra_fields: {{ .Values.log.extraFields | toJson }}
ssh_service:
  enabled: false
auth_service:
  enabled: false
proxy_service:
  enabled: true
{{- if .Values.publicAddr }}
  public_addr: {{- toYaml .Values.publicAddr | nindent 8 }}
{{- else }}
  public_addr: '{{ required "clusterName is required in chart values" .Values.clusterName }}:443'
{{- end }}
{{- if ne .Values.proxyListenerMode "multiplex" }}
  listen_addr: 0.0.0.0:3023
  {{- if .Values.sshPublicAddr }}
  ssh_public_addr: {{- toYaml .Values.sshPublicAddr | nindent 8 }}
  {{- end }}
  tunnel_listen_addr: 0.0.0.0:3024
  {{- if .Values.tunnelPublicAddr }}
  tunnel_public_addr: {{- toYaml .Values.tunnelPublicAddr | nindent 8 }}
  {{- end }}
  kube_listen_addr: 0.0.0.0:3026
  {{- if .Values.kubePublicAddr }}
  kube_public_addr: {{- toYaml .Values.kubePublicAddr | nindent 8 }}
  {{- end }}
  mysql_listen_addr: 0.0.0.0:3036
  {{- if .Values.mysqlPublicAddr }}
  mysql_public_addr: {{- toYaml .Values.mysqlPublicAddr | nindent 8 }}
  {{- end }}
  {{- if .Values.separatePostgresListener }}
  postgres_listen_addr: 0.0.0.0:5432
    {{- if .Values.postgresPublicAddr }}
  postgres_public_addr: {{- toYaml .Values.postgresPublicAddr | nindent 8 }}
    {{- else }}
  postgres_public_addr: {{ .Values.clusterName }}:5432
    {{- end }}
  {{- end }}
  {{- if .Values.separateMongoListener }}
  mongo_listen_addr: 0.0.0.0:27017
    {{- if .Values.mongoPublicAddr }}
  mongo_public_addr: {{- toYaml .Values.mongoPublicAddr | nindent 8 }}
    {{- else }}
  mongo_public_addr: {{ .Values.clusterName }}:27017
    {{- end }}
  {{- end }}
{{- end }}
{{- if or .Values.highAvailability.certManager.enabled .Values.tls.existingSecretName }}
  https_keypairs:
  - key_file: /etc/teleport-tls/tls.key
    cert_file: /etc/teleport-tls/tls.crt
  https_keypairs_reload_interval: 12h
{{- else if .Values.acme }}
  acme:
    enabled: {{ .Values.acme }}
    email: {{ required "acmeEmail is required in chart values" .Values.acmeEmail }}
  {{- if .Values.acmeURI }}
    uri: {{ .Values.acmeURI }}
  {{- end }}
{{- end }}
{{- if .Values.proxyProtocol }}
  proxy_protocol: {{ .Values.proxyProtocol | quote }}
{{- end }}
{{- if and .Values.ingress.enabled (semverCompare ">= 14.0.0-0" (include "teleport-cluster.version" .)) }}
  trust_x_forwarded_for: true
{{- end }}
{{- end -}}
