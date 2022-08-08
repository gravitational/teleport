{{- define "teleport.kube.agent.isUpgrade" -}}
{{- /* Checks if action is an upgrade from an old release that didn't support Secret storage */}}
{{- if and .Release.IsUpgrade  }}
  {{- $deployment := (lookup "apps/v1" "Deployment"  .Release.Namespace .Release.Name ) -}}
  {{- if ($deployment) }}
true
  {{- else if .Values.unitTestUpgrade }}
true
  {{- end }}
{{- end }}
{{- end -}}