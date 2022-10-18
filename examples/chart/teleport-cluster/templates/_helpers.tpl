{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}
{{- end -}}
{{- define "teleport.clusterName" -}}
{{ coalesce .Values.clusterName ( printf "%s.%s.svc.cluster.local" .Release.Name .Release.Namespace ) }}
{{- end -}}
