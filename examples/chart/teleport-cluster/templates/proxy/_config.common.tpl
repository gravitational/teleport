{{- define "teleport-cluster.proxy.config.common" -}}
{{- $proxy := mustMergeOverwrite (mustDeepCopy .Values) .Values.proxy -}}
{{- $logLevel := (coalesce $proxy.logLevel $proxy.log.level "INFO") -}}
version: v3
teleport:
  join_params:
    method: kubernetes
    token_name: "{{.Release.Name}}-proxy"
  auth_server: "{{ include "teleport-cluster.auth.serviceFQDN" . }}:3025"
  log:
    severity: {{ $logLevel }}
    output: {{ $proxy.log.output }}
    format:
      output: {{ $proxy.log.format }}
      extra_fields: {{ $proxy.log.extraFields | toJson }}
ssh_service:
  enabled: false
auth_service:
  enabled: false
proxy_service:
  enabled: true
{{- if $proxy.publicAddr }}
  public_addr: {{- toYaml $proxy.publicAddr | nindent 8 }}
{{- else }}
  public_addr: '{{ required "clusterName is required in chart values" $proxy.clusterName }}:443'
{{- end }}
{{- if ne $proxy.proxyListenerMode "multiplex" }}
  listen_addr: 0.0.0.0:3023
  {{- if $proxy.sshPublicAddr }}
  ssh_public_addr: {{- toYaml $proxy.sshPublicAddr | nindent 8 }}
  {{- end }}
  tunnel_listen_addr: 0.0.0.0:3024
  {{- if $proxy.tunnelPublicAddr }}
  tunnel_public_addr: {{- toYaml $proxy.tunnelPublicAddr | nindent 8 }}
  {{- end }}
  kube_listen_addr: 0.0.0.0:3026
  {{- if $proxy.kubePublicAddr }}
  kube_public_addr: {{- toYaml $proxy.kubePublicAddr | nindent 8 }}
  {{- end }}
  mysql_listen_addr: 0.0.0.0:3036
  {{- if $proxy.mysqlPublicAddr }}
  mysql_public_addr: {{- toYaml $proxy.mysqlPublicAddr | nindent 8 }}
  {{- end }}
  {{- if $proxy.separatePostgresListener }}
  postgres_listen_addr: 0.0.0.0:5432
    {{- if $proxy.postgresPublicAddr }}
  postgres_public_addr: {{- toYaml $proxy.postgresPublicAddr | nindent 8 }}
    {{- else }}
  postgres_public_addr: {{ $proxy.clusterName }}:5432
    {{- end }}
  {{- end }}
  {{- if $proxy.separateMongoListener }}
  mongo_listen_addr: 0.0.0.0:27017
    {{- if $proxy.mongoPublicAddr }}
  mongo_public_addr: {{- toYaml $proxy.mongoPublicAddr | nindent 8 }}
    {{- else }}
  mongo_public_addr: {{ $proxy.clusterName }}:27017
    {{- end }}
  {{- end }}
{{- end }}
{{- if or $proxy.highAvailability.certManager.enabled $proxy.tls.existingSecretName }}
  https_keypairs:
  - key_file: /etc/teleport-tls/tls.key
    cert_file: /etc/teleport-tls/tls.crt
  https_keypairs_reload_interval: 12h
{{- else if $proxy.acme }}
  acme:
    enabled: {{ $proxy.acme }}
    email: {{ required "acmeEmail is required in chart values" $proxy.acmeEmail }}
  {{- if $proxy.acmeURI }}
    uri: {{ $proxy.acmeURI }}
  {{- end }}
{{- end }}
{{- if $proxy.proxyProtocol }}
  proxy_protocol: {{ $proxy.proxyProtocol | quote }}
{{- end }}
{{- if and $proxy.ingress.enabled (semverCompare ">= 14.0.0-0" (include "teleport-cluster.version" .)) }}
  trust_x_forwarded_for: true
{{- end }}
{{- end -}}
