{{- define "teleport-proxy.join-token-secret" -}}
{{- $proxy := mustDeepCopy . -}}
{{- $_ := set $proxy "Values" (mustMergeOverwrite (mustDeepCopy .Values) .Values.proxy) -}}
{{- if and $proxy.Values.joinTokenSecret.create (eq $proxy.Values.join_params.method "token") -}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $proxy.Values.joinTokenSecret.name }}
  namespace: {{ $proxy.Release.Namespace }}
{{- if $proxy.Values.extraLabels.joinTokenSecretSecret }}
  labels:
  {{- toYaml $proxy.Values.extraLabels.joinTokenSecret | nindent 4 }}
{{- end }}
{{- if $proxy.Values.annotations.joinTokenSecret}}
  annotations:
  {{- toYaml $proxy.Values.annotations.joinTokenSecret | nindent 4 }}
{{- end }}
type: Opaque
stringData:
  auth-token: |
    {{ $proxy.Values.join_params.token_name }}
{{- end -}}
{{- end -}}
