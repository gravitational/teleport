{{- define "teleport-cluster.auth.config.aws" -}}
{{ mustMergeOverwrite (include "teleport-cluster.auth.config.common" . | fromYaml) (include "teleport-cluster.auth.config.aws.overrides" . | fromYaml) | toYaml }}
{{- end -}}

{{- define "teleport-cluster.auth.config.aws.overrides" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
teleport:
  storage:
    type: dynamodb
    region: {{ required "aws.region is required in chart values" $auth.aws.region }}
    table_name: {{ required "aws.backendTable is required in chart values" $auth.aws.backendTable }}
    audit_events_uri: {{- include "teleport-cluster.auth.config.aws.audit" . | nindent 4 }}
    audit_sessions_uri: s3://{{ required "aws.sessionRecordingBucket is required in chart values" $auth.aws.sessionRecordingBucket }}
    continuous_backups: {{ required "aws.backups is required in chart values" $auth.aws.backups }}
  {{- if $auth.aws.dynamoAutoScaling }}
    auto_scaling: true
    billing_mode: provisioned
    read_min_capacity: {{ required "aws.readMinCapacity is required when aws.dynamoAutoScaling is true" $auth.aws.readMinCapacity }}
    read_max_capacity: {{ required "aws.readMaxCapacity is required when aws.dynamoAutoScaling is true" $auth.aws.readMaxCapacity }}
    read_target_value: {{ required "aws.readTargetValue is required when aws.dynamoAutoScaling is true" $auth.aws.readTargetValue }}
    write_min_capacity: {{ required "aws.writeMinCapacity is required when aws.dynamoAutoScaling is true" $auth.aws.writeMinCapacity }}
    write_max_capacity: {{ required "aws.writeMaxCapacity is required when aws.dynamoAutoScaling is true" $auth.aws.writeMaxCapacity }}
    write_target_value: {{ required "aws.writeTargetValue is required when aws.dynamoAutoScaling is true" $auth.aws.writeTargetValue }}
  {{- else }}
    auto_scaling: false
  {{- end }}
  {{- if $auth.aws.accessMonitoring.enabled }}
    {{- if not $auth.aws.athenaURL }}
      {{- fail "AccessMonitoring requires an Athena Event backend" }}
    {{- end }}
auth_service:
  access_monitoring:
    enabled: true
    report_results: {{ $auth.aws.accessMonitoring.reportResults | quote }}
    role_arn: {{ $auth.aws.accessMonitoring.roleARN | quote }}
    workgroup: {{ $auth.aws.accessMonitoring.workgroup | quote }}
  {{- end }}
{{- end -}}

{{- define "teleport-cluster.auth.config.aws.audit" -}}
{{- $auth := mustMergeOverwrite (mustDeepCopy .Values) .Values.auth -}}
  {{- if and $auth.aws.auditLogTable (not $auth.aws.athenaURL) -}}
- 'dynamodb://{{$auth.aws.auditLogTable}}'
  {{- else if and (not $auth.aws.auditLogTable) $auth.aws.athenaURL -}}
- {{ $auth.aws.athenaURL | quote }}
  {{- else if and $auth.aws.auditLogTable $auth.aws.athenaURL -}}
    {{- if eq $auth.aws.auditLogPrimaryBackend "dynamo" -}}
- 'dynamodb://{{$auth.aws.auditLogTable}}'
- {{ $auth.aws.athenaURL | quote }}
    {{- else if eq $auth.aws.auditLogPrimaryBackend "athena" -}}
- {{ $auth.aws.athenaURL | quote }}
- 'dynamodb://{{$auth.aws.auditLogTable}}'
    {{- else -}}
      {{- fail "Both Dynamo and Athena audit backends are enabled. You must specify the primary backend by setting `aws.auditLogPrimaryBackend` to either 'dynamo' or 'athena'." -}}
    {{- end -}}
  {{- else -}}
    {{- fail "You need an audit backend. In AWS mode, you must set at least one of `aws.auditLogTable` (Dynamo) and `aws.athenaURL` (Athena)." -}}
  {{- end -}}
  {{- if $auth.aws.auditLogMirrorOnStdout }}
- 'stdout://'
  {{- end -}}
{{- end -}}
