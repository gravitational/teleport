{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-proxy.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}
{{- end -}}

{{- define "teleport-proxy.auth_server" -}}
{{- $auth := required "'auth_server' is required" .Values.auth_server -}}
{{ regexMatch ":[0-9]+$" $auth | ternary $auth (printf "%s:3025" $auth) -}}
{{- end -}}

{{- define "teleport-proxy.validate" -}}
{{- if empty .Values.join_params.method -}}
{{- fail "join_params.method is required" -}}
{{- end -}}
{{- if empty .Values.join_params.token_name -}}
{{- fail "join_params.token_name is required" -}}
{{- end -}}
{{- end -}}
