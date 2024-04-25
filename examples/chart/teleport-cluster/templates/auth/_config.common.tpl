{{- define "teleport-cluster.auth.config.common" -}}
{{- $authentication := mustMergeOverwrite .Values.authentication (default dict .Values.authenticationSecondFactor) -}}
{{- $logLevel := (coalesce .Values.logLevel .Values.log.level "INFO") -}}
version: v3
kubernetes_service:
  enabled: true
  listen_addr: 0.0.0.0:3026
  public_addr: "{{ include "teleport-cluster.auth.serviceFQDN" . }}:3026"
{{- if .Values.kubeClusterName }}
  kube_cluster_name: {{ .Values.kubeClusterName }}
{{- else }}
  kube_cluster_name: {{ .Values.clusterName }}
{{- end }}
{{- if .Values.labels }}
  labels: {{- toYaml .Values.labels | nindent 8 }}
{{- end }}
proxy_service:
  enabled: false
ssh_service:
  enabled: false
auth_service:
  enabled: true
  cluster_name: {{ required "clusterName is required in chart values" .Values.clusterName }}
{{- if .Values.enterprise }}
  license_file: '/var/lib/license/license.pem'
{{- end }}
  authentication:
    type: "{{ required "authentication.type is required in chart values" (coalesce .Values.authenticationType $authentication.type) }}"
    local_auth: {{ $authentication.localAuth }}
{{- if $authentication.connectorName }}
    connector_name: "{{ $authentication.connectorName }}"
{{- end }}
{{- if $authentication.lockingMode }}
    locking_mode: "{{ $authentication.lockingMode }}"
{{- end }}
{{- if $authentication.secondFactor }}
    second_factor: "{{ $authentication.secondFactor }}"
  {{- if not (or (eq $authentication.secondFactor "off") (eq $authentication.secondFactor "otp")) }}
    webauthn:
      rp_id: {{ required "clusterName is required in chart values" .Values.clusterName }}
    {{- if $authentication.webauthn }}
      {{- if $authentication.webauthn.attestationAllowedCas }}
      attestation_allowed_cas: {{- toYaml $authentication.webauthn.attestationAllowedCas | nindent 12 }}
      {{- end }}
      {{- if $authentication.webauthn.attestationDeniedCas }}
      attestation_denied_cas: {{- toYaml $authentication.webauthn.attestationDeniedCas | nindent 12 }}
      {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
{{- if .Values.sessionRecording }}
  session_recording: {{ .Values.sessionRecording | squote }}
{{- end }}
{{- if .Values.proxyListenerMode }}
  proxy_listener_mode: {{ .Values.proxyListenerMode }}
{{- end }}
teleport:
  auth_server: 127.0.0.1:3025
  log:
    severity: {{ $logLevel }}
    output: {{ .Values.log.output }}
    format:
      output: {{ .Values.log.format }}
      extra_fields: {{ .Values.log.extraFields | toJson }}
{{- end -}}
