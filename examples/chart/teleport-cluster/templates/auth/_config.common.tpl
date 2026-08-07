{{- define "teleport-cluster.auth.config.common" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
{{- $authentication := mustMergeOverwrite $auth.authentication (default dict $auth.authenticationSecondFactor) -}}
{{- $logLevel := (coalesce $auth.logLevel $auth.log.level "INFO") -}}
version: v3
kubernetes_service:
  enabled: true
  listen_addr: 0.0.0.0:3026
  public_addr: "{{ include "teleport-cluster.auth.serviceFQDN" . }}:3026"
{{- if $auth.kubeClusterName }}
  kube_cluster_name: {{ $auth.kubeClusterName }}
{{- else }}
  kube_cluster_name: {{ $auth.clusterName }}
{{- end }}
{{- if $auth.labels }}
  labels: {{- toYaml $auth.labels | nindent 8 }}
{{- end }}
proxy_service:
  enabled: false
ssh_service:
  enabled: false
auth_service:
  enabled: true
  cluster_name: {{ required "clusterName is required in chart values" $auth.clusterName }}
{{- if $auth.enterprise }}
  license_file: '/var/lib/license/license.pem'
{{- end }}
  authentication:
    type: "{{ required "authentication.type is required in chart values" (coalesce $auth.authenticationType $authentication.type) }}"
    local_auth: {{ $authentication.localAuth }}
{{- if $authentication.passwordless }}
    passwordless: {{ $authentication.passwordless }}
{{- end }}
{{- if $authentication.connectorName }}
    connector_name: "{{ $authentication.connectorName }}"
{{- end }}
{{- if $authentication.lockingMode }}
    locking_mode: "{{ $authentication.lockingMode }}"
{{- end }}
{{- $hasWebauthnMFA := false }}
{{/* secondFactor takes precedence for backward compatibility, but new chart releases
should have second_factor unset and privilege second_factors instead.
Sadly, it is not possible to do a conversion between second_factor and second_factors
because of the "off" value. */}}
{{- if $authentication.secondFactor }}
    second_factor: {{ $authentication.secondFactor | squote }}
    {{- if has $authentication.secondFactor (list "webauthn" "on" "optional") }}
      {{- $hasWebauthnMFA = true }}
    {{- end }}
{{- else }}
    second_factors: {{- toYaml $authentication.secondFactors | nindent 6 }}
    {{- if has "webauthn" $authentication.secondFactors }}
      {{- $hasWebauthnMFA = true }}
    {{- end }}
{{- end }}
{{- if $hasWebauthnMFA }}
    webauthn:
      rp_id: {{ required "clusterName is required in chart values" $auth.clusterName }}
      {{- if $authentication.webauthn }}
        {{- if $authentication.webauthn.attestationAllowedCas }}
      attestation_allowed_cas: {{- toYaml $authentication.webauthn.attestationAllowedCas | nindent 12 }}
        {{- end }}
        {{- if $authentication.webauthn.attestationDeniedCas }}
      attestation_denied_cas: {{- toYaml $authentication.webauthn.attestationDeniedCas | nindent 12 }}
        {{- end }}
      {{- end }}
{{- end }}
{{- if $auth.sessionRecording }}
  session_recording: {{ $auth.sessionRecording | squote }}
{{- end }}
{{- if $auth.proxyListenerMode }}
  proxy_listener_mode: {{ $auth.proxyListenerMode }}
{{- end }}
teleport:
  auth_server: 127.0.0.1:3025
  log:
    severity: {{ $logLevel }}
    output: {{ $auth.log.output }}
    format:
      output: {{ $auth.log.format }}
      extra_fields: {{ $auth.log.extraFields | toJson }}
{{- end -}}
