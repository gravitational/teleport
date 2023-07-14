{{- define "teleport-cluster.auth.config.aws" -}}
{{ include "teleport-cluster.auth.config.common" . }}
  storage:
    type: dynamodb
    region: {{ required "aws.region is required in chart values" .Values.aws.region }}
    table_name: {{ required "aws.backendTable is required in chart values" .Values.aws.backendTable }}
    {{- if .Values.aws.auditLogMirrorOnStdout }}
    audit_events_uri: ['dynamodb://{{ required "aws.auditLogTable is required in chart values" .Values.aws.auditLogTable }}', 'stdout://']
    {{- else }}
    audit_events_uri: ['dynamodb://{{ required "aws.auditLogTable is required in chart values" .Values.aws.auditLogTable }}']
    {{- end }}
    audit_sessions_uri: s3://{{ required "aws.sessionRecordingBucket is required in chart values" .Values.aws.sessionRecordingBucket }}
    continuous_backups: {{ required "aws.backups is required in chart values" .Values.aws.backups }}
    {{- if .Values.aws.dynamoAutoScaling }}
    auto_scaling: true
    read_min_capacity: {{ required "aws.readMinCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.readMinCapacity }}
    read_max_capacity: {{ required "aws.readMaxCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.readMaxCapacity }}
    read_target_value: {{ required "aws.readTargetValue is required when aws.dynamoAutoScaling is true" .Values.aws.readTargetValue }}
    write_min_capacity: {{ required "aws.writeMinCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.writeMinCapacity }}
    write_max_capacity: {{ required "aws.writeMaxCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.writeMaxCapacity }}
    write_target_value: {{ required "aws.writeTargetValue is required when aws.dynamoAutoScaling is true" .Values.aws.writeTargetValue }}
    {{- else }}
    auto_scaling: false
    {{- end }}
  {{ if eq .Values.aws.auditLogTable .Values.aws.backendTable  }}
    {{- fail "aws.auditLogTable and aws.backendTable must not be the same table" }}
  {{- end -}}    
{{- end -}}
