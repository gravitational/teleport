{{- /*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/ -}}

{{- define "teleport-kube-updater.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "teleport-kube-updater.fullname" -}}
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

{{- define "teleport-kube-updater.namespace" -}}
{{- default .Release.Namespace .Values.namespaceOverride }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "teleport-kube-updater.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- printf "%s-updater" (include "teleport-kube-updater.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "teleport-kube-updater.app.fullname" -}}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}



{{/**/}}
{{/*Create the name of the service account to use*/}}
{{/*if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name*/}}
{{/**/}}
{{/*{{- define "teleport-kube-updater.serviceAccountName" -}}*/}}
{{/*{{- coalesce .Values.serviceAccount.name .Values.serviceAccountName .Release.Name -}}*/}}
{{/*{{- end -}}*/}}


{{- define "teleport-kube-updater.version" -}}
{{- if .Values.teleportVersionOverride -}}
  {{- .Values.teleportVersionOverride -}}
{{- else -}}
  {{- .Chart.Version -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-updater.baseImage" -}}
{{- if .Values.enterprise -}}
  {{- .Values.enterpriseImage -}}
{{- else -}}
  {{- .Values.image -}}
{{- end -}}
{{- end -}}

{{- define "teleport-kube-updater.image" -}}
{{ include "teleport-kube-updater.baseImage" . }}:{{ include "teleport-kube-updater.version" . }}
{{- end -}}
