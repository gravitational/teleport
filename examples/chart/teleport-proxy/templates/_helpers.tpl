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
{{- $boundKeypair := default (dict) .Values.join_params.bound_keypair -}}
{{- $secretValue := get $boundKeypair "registration_secret_value" -}}
{{- $secretPath := get $boundKeypair "registration_secret_path" -}}
{{- $staticKeyPath := get $boundKeypair "static_key_path" -}}
{{- if $secretValue -}}
  {{- if and $secretValue $secretPath -}}
{{- fail "join_params.bound_keypair.registration_secret_value and join_params.bound_keypair.registration_secret_path are mutually exclusive" -}}
  {{- end -}}
  {{- if and $secretValue $staticKeyPath -}}
{{- fail "join_params.bound_keypair.registration_secret_value and join_params.bound_keypair.static_key_path are mutually exclusive" -}}
  {{- end -}}
  {{- if and .Release.IsUpgrade .Values.validateConfigOnDeploy (not (lookup "v1" "Secret" .Release.Namespace (include "teleport-proxy.bound-keypair-registration-secret-name" .))) -}}
{{- fail (printf "upgrading with join_params.bound_keypair.registration_secret_value set and validateConfigOnDeploy=true requires the Secret %q to already exist before pre-upgrade hooks run" (include "teleport-proxy.bound-keypair-registration-secret-name" .)) -}}
  {{- end -}}
{{- end -}}
{{- if and $secretPath $staticKeyPath -}}
{{- fail "join_params.bound_keypair.registration_secret_path and join_params.bound_keypair.static_key_path are mutually exclusive" -}}
{{- end -}}
{{- end -}}

{{- define "teleport-proxy.join-token-mount-path" -}}
/etc/teleport-secrets/auth-token
{{- end -}}

{{- define "teleport-proxy.bound-keypair-registration-secret-mount-path" -}}
/etc/teleport-secrets/registration-secret
{{- end -}}

{{- define "teleport-proxy.bound-keypair-registration-secret-name" -}}
{{- printf "%s-bound-keypair-registration-secret" .Release.Name -}}
{{- end -}}

{{- define "teleport-proxy.token_name" -}}
  {{- if eq .Values.join_params.method "token" -}}
     {{- include "teleport-proxy.join-token-mount-path" . -}}
  {{- else -}}
    {{- .Values.join_params.token_name -}}
  {{- end -}}
{{- end -}}

{{- define "teleport-proxy.uses-bound-keypair-registration-secret" -}}
{{- $boundKeypair := default (dict) .Values.join_params.bound_keypair -}}
{{- if get $boundKeypair "registration_secret_value" -}}
true
{{- end -}}
{{- end -}}

{{- define "teleport-proxy.extraVolumes" -}}
  {{- $extraVolumes := .Values.extraVolumes -}}
  {{- if eq .Values.join_params.method "token" -}}
    {{- $extraVolumes = append $extraVolumes (include "teleport-proxy.join-token-volume" . | fromYaml) -}}
  {{- end -}}
  {{- if include "teleport-proxy.uses-bound-keypair-registration-secret" . -}}
    {{- $extraVolumes = append $extraVolumes (include "teleport-proxy.bound-keypair-registration-secret-volume" . | fromYaml) -}}
  {{- end -}}
  {{- $extraVolumes | toYaml -}}
{{- end -}}

{{- define "teleport-proxy.extraVolumeMounts" -}}
  {{- $extraVolumeMounts := .Values.extraVolumeMounts -}}
  {{- if eq .Values.join_params.method "token" -}}
    {{- $extraVolumeMounts = append $extraVolumeMounts (include "teleport-proxy.join-token-volume-mount" . | fromYaml) -}}
  {{- end -}}
  {{- if include "teleport-proxy.uses-bound-keypair-registration-secret" . -}}
    {{- $extraVolumeMounts = append $extraVolumeMounts (include "teleport-proxy.bound-keypair-registration-secret-volume-mount" . | fromYaml) -}}
  {{- end -}}
  {{- $extraVolumeMounts | toYaml -}}
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
subPath: auth-token
{{- end -}}

{{- define "teleport-proxy.bound-keypair-registration-secret-volume" -}}
name: "bound-keypair-registration-secret"
secret:
  secretName: {{ include "teleport-proxy.bound-keypair-registration-secret-name" . | quote }}
{{- end -}}

{{- define "teleport-proxy.bound-keypair-registration-secret-volume-mount" -}}
mountPath: {{ include "teleport-proxy.bound-keypair-registration-secret-mount-path" . | quote }}
name: "bound-keypair-registration-secret"
readOnly: true
subPath: registration-secret
{{- end -}}
