{{/*
  auth_server and join_params are treated special only because they're
  required.
*/}}
{{- define "teleport-proxy.generatedTeleportConfig" -}}
{{- $joinParams := mustDeepCopy .Values.join_params -}}
{{- $_ := set $joinParams "token_name" (include "teleport-proxy.token_name" .) -}}
{{- if include "teleport-proxy.uses-bound-keypair-registration-secret" . -}}
  {{- $boundKeypair := mustDeepCopy (default (dict) (get $joinParams "bound_keypair")) -}}
  {{- $_ := unset $boundKeypair "registration_secret_value" -}}
  {{- $_ := set $boundKeypair "registration_secret_path" (include "teleport-proxy.bound-keypair-registration-secret-mount-path" .) -}}
  {{- $_ := set $joinParams "bound_keypair" $boundKeypair -}}
{{- end -}}
teleport:
  auth_server: {{ include "teleport-proxy.auth_server" . | quote }}
  join_params:
{{- toYaml $joinParams | nindent 4 }}
{{- end -}}

{{- define "teleport-proxy.teleportConfig" -}}
{{- $generated := include "teleport-proxy.generatedTeleportConfig" . | fromYaml -}}
{{- $user := deepCopy (default dict .Values.teleportConfig) -}}
{{- $teleportConfig := mergeOverwrite $generated $user -}}
{{- toYaml $teleportConfig -}}
{{- end -}}
