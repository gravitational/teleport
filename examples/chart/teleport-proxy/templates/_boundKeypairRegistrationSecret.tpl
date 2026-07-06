{{- define "teleport-proxy.bound-keypair-registration-secret" -}}
{{- $proxy := mustDeepCopy . -}}
{{- $_ := set $proxy "Values" (mustMergeOverwrite (mustDeepCopy .Values) .Values.proxy) -}}
{{- if include "teleport-proxy.uses-bound-keypair-registration-secret" $proxy -}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "teleport-proxy.bound-keypair-registration-secret-name" $proxy }}
  namespace: {{ $proxy.Release.Namespace }}
{{- if $proxy.Values.extraLabels.secret }}
  labels:
  {{- toYaml $proxy.Values.extraLabels.secret | nindent 4 }}
  {{- end }}
{{- if $proxy.Values.annotations.secret }}
  annotations:
  {{- toYaml $proxy.Values.annotations.secret | nindent 4 }}
{{- end }}
type: Opaque
stringData:
  registration-secret: |
    {{ get (default (dict) $proxy.Values.join_params.bound_keypair) "registration_secret_value" }}
{{- end -}}
{{- end -}}
