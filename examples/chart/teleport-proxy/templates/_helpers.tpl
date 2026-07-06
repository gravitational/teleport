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
{{- if or (eq .Values.join_params.method "token") (eq .Values.join_params.method "bound_keypair") -}}
{{- fail "Secret join methods ('token' and 'bound_keypair') aren't supported by this Helm chart. Please consider the more secure delegated join methods. See https://goteleport.com/docs/reference/deployment/join-methods/" -}}
{{- else if empty .Values.join_params.token_name -}}
{{- fail "join_params.token_name is required" -}}
{{- end -}}
{{- end -}}