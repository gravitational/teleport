{{- define "teleport-kube-updater.serviceaccount" -}}
{{- if .serviceAccount.create -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .serviceAccount.name }}
  namespace: {{ .releaseNamespace }}
{{- if .extraLabels.serviceAccount }}
  labels: {{- toYaml .extraLabels.serviceAccount | nindent 4 }}
{{- end }}
{{- if .annotations.serviceAccount }}
  annotations: {{- toYaml .annotations.serviceAccount | nindent 4 }}
{{- end -}}
{{- end -}}
{{- end -}}
