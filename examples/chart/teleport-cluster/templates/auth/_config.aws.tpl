{{- define "teleport-cluster.auth.config.aws" -}}
{{ mustMergeOverwrite (include "teleport-cluster.auth.config.common" . | fromYaml) (include "teleport-cluster.auth.config.aws.overrides" . | fromYaml) | toYaml }}
{{- end -}}

{{- define "teleport-cluster.auth.config.aws.overrides" -}}
teleport:
  storage:
    type: dynamodb
    region: {{ required "aws.region is required in chart values" .Values.aws.region }}
    table_name: {{ required "aws.backendTable is required in chart values" .Values.aws.backendTable }}
    audit_events_uri: {{- include "teleport-cluster.auth.config.aws.audit" . | nindent 4 }}
    audit_sessions_uri: s3://{{ required "aws.sessionRecordingBucket is required in chart values" .Values.aws.sessionRecordingBucket }}
    continuous_backups: {{ required "aws.backups is required in chart values" .Values.aws.backups }}
  {{- if .Values.aws.dynamoAutoScaling }}
    auto_scaling: true
    billing_mode: provisioned
    read_min_capacity: {{ required "aws.readMinCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.readMinCapacity }}
    read_max_capacity: {{ required "aws.readMaxCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.readMaxCapacity }}
    read_target_value: {{ required "aws.readTargetValue is required when aws.dynamoAutoScaling is true" .Values.aws.readTargetValue }}
    write_min_capacity: {{ required "aws.writeMinCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.writeMinCapacity }}
    write_max_capacity: {{ required "aws.writeMaxCapacity is required when aws.dynamoAutoScaling is true" .Values.aws.writeMaxCapacity }}
    write_target_value: {{ required "aws.writeTargetValue is required when aws.dynamoAutoScaling is true" .Values.aws.writeTargetValue }}
  {{- else }}
    auto_scaling: false
  {{- end }}
  {{- if .Values.aws.accessMonitoring.enabled }}
    {{- if not .Values.aws.athenaURL }}
      {{- fail "AccessMonitoring requires an Athena Event backend" }}
    {{- end }}
auth_service:
  access_monitoring:
    enabled: true
    report_results: {{ .Values.aws.accessMonitoring.reportResults | quote }}
    role_arn: {{ .Values.aws.accessMonitoring.roleARN | quote }}
    workgroup: {{ .Values.aws.accessMonitoring.workgroup | quote }}
  {{- end }}
{{- end -}}

{{- define "teleport-cluster.auth.config.aws.audit" -}}
  {{- if and .Values.aws.auditLogTable (not .Values.aws.athenaURL) -}}
- 'dynamodb://{{.Values.aws.auditLogTable}}'
  {{- else if and (not .Values.aws.auditLogTable) .Values.aws.athenaURL -}}
- {{ .Values.aws.athenaURL | quote }}
  {{- else if and .Values.aws.auditLogTable .Values.aws.athenaURL -}}
    {{- if eq .Values.aws.auditLogPrimaryBackend "dynamo" -}}
- 'dynamodb://{{.Values.aws.auditLogTable}}'
- {{ .Values.aws.athenaURL | quote }}
    {{- else if eq .Values.aws.auditLogPrimaryBackend "athena" -}}
- {{ .Values.aws.athenaURL | quote }}
- 'dynamodb://{{.Values.aws.auditLogTable}}'
    {{- else -}}
      {{- fail "Both Dynamo and Athena audit backends are enabled. You must specify the primary backend by setting `aws.auditLogPrimaryBackend` to either 'dynamo' or 'athena'." -}}
    {{- end -}}
  {{- else -}}
    {{- fail "You need an audit backend. In AWS mode, you must set at least one of `aws.auditLogTable` (Dynamo) and `aws.athenaURL` (Athena)." -}}
  {{- end -}}
  {{- if .Values.aws.auditLogMirrorOnStdout }}
- 'stdout://'
  {{- end -}}
{{- end -}}
