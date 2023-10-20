{{/* Helper to build the database connection string, adds paraneters if needed */}}
{{- define "teleport-cluster.auth.config.azure.conn_string.query" }}
  {{- if .Values.azure.databasePoolMaxConnections -}}
    {{- printf "sslmode=verify-full&pool_max_conns=%v" .Values.azure.databasePoolMaxConnections -}}
  {{- else -}}
    sslmode=verify-full
  {{- end -}}
{{- end -}}

{{- define "teleport-cluster.auth.config.azure" -}}
{{ include "teleport-cluster.auth.config.common" . }}
  storage:
    type: postgresql
    auth_mode: azure
    conn_string: {{ urlJoin (dict
      "scheme" "postgresql"
      "userinfo" .Values.azure.databaseUser
      "host" .Values.azure.databaseHost
      "path" .Values.azure.backendDatabase
      "query" (include "teleport-cluster.auth.config.azure.conn_string.query" .)
    ) | toYaml }}
    audit_sessions_uri: {{ urlJoin (dict
      "scheme" "azblob"
      "host" .Values.azure.sessionRecordingStorageAccount
    ) | toYaml }}
    audit_events_uri:
      - {{ urlJoin (dict
          "scheme" "postgresql"
          "userinfo" .Values.azure.databaseUser
          "host" .Values.azure.databaseHost
          "path" .Values.azure.auditLogDatabase
          "query" "sslmode=verify-full"
          "fragment" "auth_mode=azure"
        ) | toYaml }}
{{- if .Values.azure.auditLogMirrorOnStdout }}
      - "stdout://"
{{- end }}
{{- end -}}
