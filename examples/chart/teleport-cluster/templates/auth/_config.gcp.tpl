{{- define "teleport-cluster.auth.config.gcp" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
{{ include "teleport-cluster.auth.config.common" . }}
  storage:
    type: firestore
    project_id: {{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}
    collection_name: {{ required "gcp.backendTable is required in chart values" $auth.gcp.backendTable }}
    {{- if $auth.gcp.credentialSecretName }}
    credentials_path: /etc/teleport-secrets/gcp-credentials.json
    {{- end }}
    {{- if $auth.gcp.auditLogMirrorOnStdout }}
    audit_events_uri: ['firestore://{{ required "gcp.auditLogTable is required in chart values" $auth.gcp.auditLogTable }}?projectID={{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}{{ empty $auth.gcp.credentialSecretName | ternary "" "&credentialsPath=/etc/teleport-secrets/gcp-credentials.json"}}', 'stdout://']
    {{- else }}
    audit_events_uri: ['firestore://{{ required "gcp.auditLogTable is required in chart values" $auth.gcp.auditLogTable }}?projectID={{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}{{ empty $auth.gcp.credentialSecretName | ternary "" "&credentialsPath=/etc/teleport-secrets/gcp-credentials.json"}}']
    {{- end }}
    audit_sessions_uri: "gs://{{ required "gcp.sessionRecordingBucket is required in chart values" $auth.gcp.sessionRecordingBucket }}?projectID={{ required "gcp.projectId is required in chart values" $auth.gcp.projectId }}{{ empty $auth.gcp.credentialSecretName | ternary "" "&credentialsPath=/etc/teleport-secrets/gcp-credentials.json"}}"
{{- end -}}
