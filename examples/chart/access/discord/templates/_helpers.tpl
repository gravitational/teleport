{{/*
Expand the name of the chart.
*/}}
{{- define "discord.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "discord.fullname" -}}
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
{{- define "discord.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "discord.labels" -}}
helm.sh/chart: {{ include "discord.chart" . }}
{{ include "discord.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "discord.selectorLabels" -}}
app.kubernetes.io/name: {{ include "discord.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "discord.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "discord.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Use tbot-managed identity secret if tbot is enabled
*/}}
{{- define "discord.identitySecretName" -}}
{{- if .Values.teleport.identitySecretName -}}
{{- .Values.teleport.identitySecretName -}}
{{- else if .Values.tbot.enabled -}}
  {{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}-out
{{- end }}
{{- end -}}

{{- define "discord.identitySecretPath" -}}
{{- if .Values.tbot.enabled -}}
identity
{{- else -}}
{{- .Values.teleport.identitySecretPath -}}
{{- end -}}
{{- end -}}

{{/*
Create the embedded tbot's service account name.
*/}}
{{- define "discord.tbot.serviceAccountName" -}}
{{- if .Values.tbot.serviceAccount.name -}}
{{- .Values.tbot.serviceAccount.name -}}
{{- else -}}
{{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}
{{- end }}
{{- end -}}

{{/*
Create the embedded tbot's token name.
*/}}
{{- define "discord.tbot.tokenName" -}}
{{- if .Values.tbot.token -}}
{{- .Values.tbot.token -}}
{{- else -}}
{{- .Release.Name }}-{{ default .Values.tbot.nameOverride "tbot" }}
{{- end -}}
{{- end -}}

{{/*
Create the namespace that Operator custom resources will be created in.
*/}}
{{- define "discord.crd.namespace" -}}
{{- if .Values.crd.namespace -}}
{{- .Values.crd.namespace -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{/*
Create the default TeleportProvisionToken join spec when using kubernetes join method.
*/}}
{{- define "discord.crd.defaultKubeJoinSpec" -}}
join_method: kubernetes
kubernetes:
  type: in_cluster
  allow:
  - service_account: "{{ .Release.Namespace }}:{{ include "discord.tbot.serviceAccountName" . }}"
{{- end -}}

{{/*
Create the full TeleportProvisionToken join spec.
*/}}
{{- define "discord.crd.tokenJoinSpec" -}}
{{/* Any overriden token spec must match tbot's join method */}}
{{- if and (hasKey .Values.crd.tokenSpecOverride "join_method") (ne .Values.crd.tokenSpecOverride.join_method .Values.tbot.joinMethod) -}}
{{- fail "crd.tokenSpecOverride.join_method must be same as tbot.joinMethod" -}}
{{- end -}}
{{- if eq .Values.tbot.joinMethod "kubernetes" -}}
{{- mustMergeOverwrite (include "discord.crd.defaultKubeJoinSpec" . | fromYaml) .Values.crd.tokenSpecOverride | toYaml -}}
{{- else -}}
  {{- if empty .Values.crd.tokenSpecOverride -}}
  {{- fail "crd.tokenSpecOverride cannot be empty in chart values" -}}
  {{- end -}}
  {{- if not (hasKey .Values.crd.tokenSpecOverride "join_method") -}}
  {{- fail "crd.tokenSpecOverride.join_method cannot be empty in chart values" -}}
  {{- end -}}
{{- .Values.crd.tokenSpecOverride | toYaml -}}
{{- end -}}
{{- end -}}
