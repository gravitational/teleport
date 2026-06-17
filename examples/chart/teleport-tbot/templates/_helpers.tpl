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
{{- define "teleport-tbot.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName .Release.Name -}}
{{- end -}}

{{/*
Create the name of the service account to use for the post-delete hook
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name-delete-hook
*/}}
{{- define "teleport-tbot.deleteHookServiceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- printf "%s-delete-hook" (include "teleport-tbot.serviceAccountName" . ) -}}
{{- else -}}
{{- (include "teleport-tbot.serviceAccountName" . ) -}}
{{- end -}}
{{- end -}}

{{- define "teleport-tbot.version" -}}
{{- if .Values.teleportVersionOverride -}}
  {{- .Values.teleportVersionOverride -}}
{{- else -}}
  {{- .Chart.Version -}}
{{- end -}}
{{- end -}}

{{- define "teleport-tbot.baseImage" -}}
{{- if .Values.enterprise -}}
  {{- .Values.enterpriseImage -}}
{{- else -}}
  {{- .Values.image -}}
{{- end -}}
{{- end -}}

{{- define "teleport-tbot.image" -}}
{{ include "teleport-tbot.baseImage" . }}:{{ include "teleport-tbot.version" . }}
{{- end -}}
