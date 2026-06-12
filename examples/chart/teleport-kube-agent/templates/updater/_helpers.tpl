{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-kube-agent-updater.serviceAccountName" -}}
{{- coalesce .Values.updater.serviceAccount.name (include "teleport-kube-agent.serviceAccountName" . | printf "%s-updater") -}}
{{- end -}}
