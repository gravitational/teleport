{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty use .Release.name
*/}}
{{- define "teleport.serviceAccountName" -}}
  {{- if ((.Values.serviceAccount).name) -}}
    {{- .Values.serviceAccount.name }}
  {{- else -}}
    {{- .Release.Name }}
  {{- end -}}
{{- end -}}
