{{/* Helper to build the database connection string, adds paraneters if needed */}}
{{- define "teleport-cluster.auth.config.azure.conn_string.query" }}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
  {{- if $auth.azure.databasePoolMaxConnections -}}
    {{- printf "sslmode=verify-full&pool_max_conns=%v" $auth.azure.databasePoolMaxConnections -}}
  {{- else -}}
    sslmode=verify-full
  {{- end -}}
{{- end -}}

{{- define "teleport-cluster.auth.config.azure" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
{{ include "teleport-cluster.auth.config.common" . }}
  storage:
    type: postgresql
    auth_mode: azure
    conn_string: {{ urlJoin (dict
      "scheme" "postgresql"
      "userinfo" $auth.azure.databaseUser
      "host" $auth.azure.databaseHost
      "path" $auth.azure.backendDatabase
      "query" (include "teleport-cluster.auth.config.azure.conn_string.query" .)
    ) | toYaml }}
    audit_sessions_uri: {{ urlJoin (dict
      "scheme" "azblob"
      "host" $auth.azure.sessionRecordingStorageAccount
    ) | toYaml }}
    audit_events_uri:
      - {{ urlJoin (dict
          "scheme" "postgresql"
          "userinfo" $auth.azure.databaseUser
          "host" $auth.azure.databaseHost
          "path" $auth.azure.auditLogDatabase
          "query" "sslmode=verify-full"
          "fragment" "auth_mode=azure"
        ) | toYaml }}
{{- if $auth.azure.auditLogMirrorOnStdout }}
      - "stdout://"
{{- end }}
{{- end -}}
