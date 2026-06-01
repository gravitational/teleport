{{- define "teleport-proxy-lib.internal.serviceaccount" }}
{{- $proxy := .Values -}}{{/* Minimizes diff for refactoring. Remove unneeded variable in next PR. */}}
{{- $projectedServiceAccountToken := semverCompare ">=1.20.0-0" .Capabilities.KubeVersion.Version }}
{{- if $proxy.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "teleport-proxy-lib.internal.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if $proxy.extraLabels.serviceAccount }}
    {{- toYaml $proxy.extraLabels.serviceAccount | nindent 4 }}
    {{- end }}
{{- if $proxy.annotations.serviceAccount }}
  annotations: {{- toYaml $proxy.annotations.serviceAccount | nindent 4 }}
{{- end -}}
{{- if $projectedServiceAccountToken }}
automountServiceAccountToken: false
{{- end }}
{{- end }}
{{- end }}{{/* serviceaccount */}}
