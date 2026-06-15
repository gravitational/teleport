{{- define "teleport-cluster.auth.config.gcp" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth }}
{{/* Building the firestore query. Always set the project ID, and optionally the database ID and the credentials path. */}}
{{- $firestoreParams := dict
    "projectID" (required "gcp.projectId is required in chart values" $auth.gcp.projectId)
}}
{{- if $auth.gcp.credentialSecretName }}
{{- $_ := set $firestoreParams "credentialsPath" "/etc/teleport-secrets/gcp-credentials.json" }}
{{- end }}
{{- if $auth.gcp.databaseId }}
{{- $_ := set $firestoreParams "databaseID" $auth.gcp.databaseId }}
{{- end }}
{{- $firestoreQueryParts := list }}
{{- range $key, $value := $firestoreParams }}
  {{- $firestoreQueryParts = append $firestoreQueryParts (printf "%s=%s" $key $value) }}
{{- end }}
{{- $firestoreURL := urlJoin (dict
    "scheme" "firestore"
    "host" (required "gcp.auditLogTable is required in chart values" $auth.gcp.auditLogTable)
    "query" (join "&" $firestoreQueryParts))
}}
{{- include "teleport-cluster.auth.config.common" . }}
  storage:
    type: firestore
    project_id: {{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}
    collection_name: {{ required "gcp.backendTable is required in chart values" $auth.gcp.backendTable }}
    {{- if $auth.gcp.databaseId }}
    database_id: {{ $auth.gcp.databaseId }}
    {{- end }}
    {{- if $auth.gcp.credentialSecretName }}
    credentials_path: /etc/teleport-secrets/gcp-credentials.json
    {{- end }}
    {{- if $auth.gcp.auditLogMirrorOnStdout }}
    audit_events_uri: ['{{$firestoreURL}}', 'stdout://']
    {{- else }}
    audit_events_uri: ['{{ $firestoreURL }}']
    {{- end }}
    audit_sessions_uri: "gs://{{ required "gcp.sessionRecordingBucket is required in chart values" $auth.gcp.sessionRecordingBucket }}?projectID={{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}{{ empty $auth.gcp.credentialSecretName | ternary "" "&credentialsPath=/etc/teleport-secrets/gcp-credentials.json"}}"
{{- end -}}
