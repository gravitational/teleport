{{- define "teleport-proxy-lib.internal.serviceaccount" }}
{{- $projectedServiceAccountToken := semverCompare ">=1.20.0-0" .Capabilities.KubeVersion.Version }}
{{- if .Values.serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "teleport-proxy-lib.internal.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.serviceAccount }}
    {{- toYaml .Values.extraLabels.serviceAccount | nindent 4 }}
    {{- end }}
{{- if .Values.annotations.serviceAccount }}
  annotations: {{- toYaml .Values.annotations.serviceAccount | nindent 4 }}
{{- end -}}
{{- if $projectedServiceAccountToken }}
automountServiceAccountToken: false
{{- end }}
{{- end }}
{{- end }}{{/* serviceaccount */}}
