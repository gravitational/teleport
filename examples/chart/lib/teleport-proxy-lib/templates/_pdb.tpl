{{- define "teleport-proxy-lib.internal.pdb" }}
{{- if .Values.highAvailability.podDisruptionBudget.enabled }}
{{- if .Capabilities.APIVersions.Has "policy/v1" }}
apiVersion: policy/v1
{{- else }}
apiVersion: policy/v1beta1
{{- end }}
kind: PodDisruptionBudget
metadata:
  name: {{ .Release.Name }}-proxy
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.podDisruptionBudget }}
    {{- toYaml .Values.extraLabels.podDisruptionBudget | nindent 4 }}
    {{- end }}
spec:
  minAvailable: {{ .Values.highAvailability.podDisruptionBudget.minAvailable }}
  selector:
    matchLabels: {{- include "teleport-proxy-lib.internal.selectorLabels" . | nindent 6 }}
{{- end }}
{{- end }}{{/* pdb */}}
