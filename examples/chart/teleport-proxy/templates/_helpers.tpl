{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-proxy.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}
{{- end -}}

{{- define "teleport-proxy.authServer" -}}
{{- $auth := required "'authServer' is required" .Values.authServer -}}
{{ regexMatch ":[0-9]+$" $auth | ternary $auth (printf "%s:3025" $auth) -}}
{{- end -}}

{{- define "teleport-proxy.validate" -}}
{{- if empty .Values.joinParams.method -}}
{{- fail "joinParams.method is required" -}}
{{- end -}}
{{- if or (eq .Values.joinParams.method "token") (eq .Values.joinParams.method "bound_keypair") -}}
{{- fail "Secret join methods ('token' and 'bound_keypair') aren't supported by this Helm chart. Please consider the more secure delegated join methods. See https://goteleport.com/docs/reference/deployment/join-methods/" -}}
{{- else if empty .Values.joinParams.tokenName -}}
{{- fail "joinParams.tokenName is required" -}}
{{- end -}}
{{- end -}}