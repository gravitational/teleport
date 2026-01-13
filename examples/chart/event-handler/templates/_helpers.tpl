{{/*
Expand the name of the chart.
*/}}
{{- define "event-handler.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "event-handler.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "event-handler.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "event-handler.labels" -}}
helm.sh/chart: {{ include "event-handler.chart" . }}
{{ include "event-handler.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "event-handler.selectorLabels" -}}
app.kubernetes.io/name: {{ include "event-handler.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "event-handler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "event-handler.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Use tbot-managed identity secret if tbot is enabled
*/}}
{{- define "event-handler.identitySecretName" -}}
{{- if .Values.teleport.identitySecretName -}}
{{- .Values.teleport.identitySecretName -}}
{{- else if .Values.tbot.enabled -}}
  {{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}-out
{{- end }}
{{- end -}}

{{- define "event-handler.identitySecretPath" -}}
{{- if .Values.tbot.enabled -}}
identity
{{- else -}}
{{- .Values.teleport.identitySecretPath -}}
{{- end -}}
{{- end -}}

{{/*
Create the embedded tbot's service account name.
*/}}
{{- define "event-handler.tbot.serviceAccountName" -}}
{{- if .Values.tbot.serviceAccount.name -}}
{{- .Values.tbot.serviceAccount.name -}}
{{- else -}}
{{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}
{{- end }}
{{- end -}}

{{/*
Create the name for TeleportProvisionToken custom resource.
*/}}
{{- define "event-handler.crd.tokenName" -}}
{{- if .Values.tbot.enabled -}}
{{- required "tbot.token cannot be empty in chart values" .Values.tbot.token -}}
{{- else -}}
{{- include "event-handler.fullname" . -}}-bot
{{- end -}}
{{- end -}}

{{/*
Create the default TeleportProvisionToken join spec when using kubernetes join method,
and tbot is enabled.
*/}}
{{- define "event-handler.crd.defaultKubeJoinSpec" -}}
join_method: kubernetes
kubernetes:
  allow:
  - service_account: "{{ .Release.Namespace }}:{{ include "event-handler.tbot.serviceAccountName" . }}"
{{- end -}}

{{/*
Create the full TeleportProvisionToken join spec.
*/}}
{{- define "event-handler.crd.tokenJoinSpec" -}}
{{- if and .Values.tbot.enabled (eq .Values.tbot.joinMethod "kubernetes") -}}
{{- mustMergeOverwrite (include "event-handler.crd.defaultKubeJoinSpec" . | fromYaml) .Values.crd.tokenJoinOverride | toYaml -}}
{{- else -}}
  {{- if empty .Values.crd.tokenJoinOverride -}}
  {{- fail "crd.tokenJoinOverride cannot be empty in chart values" -}}
  {{- end -}}
  {{- if not (hasKey .Values.crd.tokenJoinOverride "join_method") -}}
  {{- fail "crd.tokenJoinOverride.join_method cannot be empty in chart values" -}}
  {{- end -}}
{{- .Values.crd.tokenJoinOverride | toYaml -}}
{{- end -}}
{{- end -}}
