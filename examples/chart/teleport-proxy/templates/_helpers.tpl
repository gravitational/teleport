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
{{- if eq .Values.join_params.method "token" -}}
  {{- if empty .Values.joinTokenSecret.name -}}
{{- fail "joinTokenSecret.name is required when join_params.method is 'token'" -}}
  {{- end -}}
  {{- if and .Values.joinTokenSecret.create (empty .Values.join_params.token_name) -}}
{{- fail "join_params.token_name should be set to the token value when join_params.method is 'token' and joinTokenSecret.create is true" -}}
  {{- end -}}
  {{- if and .Release.IsUpgrade .Values.validateConfigOnDeploy .Values.joinTokenSecret.create (not (lookup "v1" "Secret" .Release.Namespace .Values.joinTokenSecret.name)) -}}
{{- fail (printf "upgrading with join_params.method='token', joinTokenSecret.create=true, and validateConfigOnDeploy=true requires the Secret %q to already exist before pre-upgrade hooks run; pre-create the Secret and set joinTokenSecret.create=false for this upgrade" .Values.joinTokenSecret.name) -}}
  {{- end -}}
{{- else if empty .Values.join_params.token_name -}}
{{- fail "join_params.token_name is required" -}}
{{- end -}}
{{- end -}}

{{- define "teleport-proxy.join-token-mount-path" -}}
/etc/teleport-secrets
{{- end -}}

{{- define "teleport-proxy.token_name" -}}
  {{- if eq .Values.join_params.method "token" -}}
     {{- printf "%s/auth-token" (include "teleport-proxy.join-token-mount-path" .) -}}
  {{- else -}}
    {{- .Values.join_params.token_name -}}
  {{- end -}}
{{- end -}}

{{- define "teleport-proxy.extraVolumes" -}}
  {{- if eq .Values.join_params.method "token" -}}
    {{- append .Values.extraVolumes (include "teleport-proxy.join-token-volume" . | fromYaml) | toYaml -}}
  {{- else -}}
    {{- .Values.extraVolumes | toYaml -}}
  {{- end -}}
{{- end -}}

{{- define "teleport-proxy.extraVolumeMounts" -}}
  {{- if eq .Values.join_params.method "token" -}}
    {{- append .Values.extraVolumeMounts (include "teleport-proxy.join-token-volume-mount" . | fromYaml) | toYaml -}}
  {{- else -}}
    {{- .Values.extraVolumeMounts | toYaml -}}
  {{- end -}}
{{- end -}}


{{- define "teleport-proxy.join-token-volume" -}}
name: "auth-token"
secret:
  secretName: {{ required "joinTokenSecret.name is required when join_params.method is 'token'" .Values.joinTokenSecret.name | quote }}
{{- end -}}

{{- define "teleport-proxy.join-token-volume-mount" -}}
mountPath: {{ include "teleport-proxy.join-token-mount-path" . | quote }}
name: "auth-token"
readOnly: true
{{- end -}}
