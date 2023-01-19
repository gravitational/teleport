{{- define "teleport.kube.agent.isUpgrade" -}}
{{- /* Checks if action is an upgrade from an old release that didn't support Secret storage */}}
{{- if .Release.IsUpgrade }}
  {{- $deployment := (lookup "apps/v1" "Deployment"  .Release.Namespace .Release.Name ) -}}
  {{- if ($deployment) }}
true
  {{- else if .Values.unitTestUpgrade }}
true
  {{- end }}
{{- end }}
{{- end -}}
{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName .Release.Name -}}
{{- end -}}
{{/*
Renders a value that contains template.
Usage:
{{ include "teleport.tplvalues.render" ( dict "value" .Values.path.to.the.Value "context" $) }}
*/}}
{{- define "teleport.tplvalues.render" -}}
  {{- if typeIs "string" .value }}
      {{- tpl .value .context }}
  {{- else }}
    {{- tpl (.value | toYaml) .context }}
  {{- end }}
{{- end -}}
