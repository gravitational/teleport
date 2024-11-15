{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-cluster.auth.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}
{{- end -}}

{{/*
Create the name of the service account to use in the auth config check hook.

If the chart is creating service accounts, we know we can create new arbitrary service accounts.
We cannot reuse the same name as the deployment SA because the non-hook service account might
not exist yet. We tried being smart with hooks but ArgoCD doesn't differentiate between install
and upgrade, causing various issues on update and eventually forcing us to use a separate SA.

If the chart is not creating service accounts, for backward compatibility we don't want
to force new service account names to existing chart users. We know the SA should already exist,
so we can use the same SA for deployments and hooks.
*/}}
{{- define "teleport-cluster.auth.hookServiceAccountName" -}}
{{- include "teleport-cluster.auth.serviceAccountName" . -}}
{{- if .Values.serviceAccount.create -}}
-hook
{{- end -}}
{{- end -}}

{{- define "teleport-cluster.proxy.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name .Release.Name -}}-proxy
{{- end -}}

{{/*
Create the name of the service account to use in the proxy config check hook.

If the chart is creating service accounts, we know we can create new arbitrary service accounts.
We cannot reuse the same name as the deployment SA because the non-hook service account might
not exist yet. We tried being smart with hooks but ArgoCD doesn't differentiate between install
and upgrade, causing various issues on update and eventually forcing us to use a separate SA.

If the chart is not creating service accounts, for backward compatibility we don't want
to force new service account names to existing chart users. We know the SA should already exist,
so we can use the same SA for deployments and hooks.
*/}}
{{- define "teleport-cluster.proxy.hookServiceAccountName" -}}
{{- include "teleport-cluster.proxy.serviceAccountName" . -}}
{{- if .Values.serviceAccount.create -}}
-hook
{{- end -}}
{{- end -}}

{{- define "teleport-cluster.version" -}}
{{- coalesce .Values.teleportVersionOverride .Chart.Version }}
{{- end -}}

{{- define "teleport-cluster.majorVersion" -}}
{{- (semver (include "teleport-cluster.version" .)).Major -}}
{{- end -}}

{{- define "teleport-cluster.previousMajorVersion" -}}
{{- sub (include "teleport-cluster.majorVersion" . | atoi ) 1 -}}
{{- end -}}

{{/* Proxy selector labels */}}
{{- define "teleport-cluster.proxy.selectorLabels" -}}
app.kubernetes.io/name: '{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'proxy'
{{- end -}}

{{/* Proxy all labels */}}
{{- define "teleport-cluster.proxy.labels" -}}
{{ include "teleport-cluster.proxy.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-cluster.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-cluster.majorVersion" . }}'
{{- end -}}

{{/* Auth pods selector labels */}}
{{- define "teleport-cluster.auth.selectorLabels" -}}
app.kubernetes.io/name: '{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'auth'
{{- end -}}

{{/* All pods all labels */}}
{{- define "teleport-cluster.labels" -}}
{{ include "teleport-cluster.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-cluster.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-cluster.majorVersion" . }}'
{{- end -}}

{{/* All pods selector labels */}}
{{- define "teleport-cluster.selectorLabels" -}}
app.kubernetes.io/name: '{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
{{- end -}}

{{/* Auth pods all labels */}}
{{- define "teleport-cluster.auth.labels" -}}
{{ include "teleport-cluster.auth.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-cluster.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-cluster.majorVersion" . }}'
{{- end -}}

{{/* ServiceNames are limited to 63 characters, we might have to truncate the ReleaseName
     to make sure the auth serviceName won't exceed this limit */}}
{{- define "teleport-cluster.auth.serviceName" -}}
{{- .Release.Name | trunc 58 | trimSuffix "-" -}}-auth
{{- end -}}

{{- define "teleport-cluster.auth.currentVersionServiceName" -}}
{{- .Release.Name | trunc 54 | trimSuffix "-" -}}-auth-v{{ include "teleport-cluster.majorVersion" . }}
{{- end -}}

{{- define "teleport-cluster.auth.previousVersionServiceName" -}}
{{- .Release.Name | trunc 54 | trimSuffix "-" -}}-auth-v{{ include "teleport-cluster.previousMajorVersion" . }}
{{- end -}}


{{/* In most places we want to use the FQDN instead of relying on Kubernetes ndots behaviour
     for performance reasons */}}
{{- define "teleport-cluster.auth.serviceFQDN" -}}
{{ include "teleport-cluster.auth.serviceName" . }}.{{ .Release.Namespace }}.svc.{{ include "teleport-cluster.clusterDomain" . }}
{{- end -}}

{{/* Returns the cluster domain if set, otherwise fallback to "cluster.local" */}}
{{- define "teleport-cluster.clusterDomain" -}}
{{ default "cluster.local" .Values.global.clusterDomain }}
{{- end -}}

{{/* Matches the operator template "teleport-cluster.operator.fullname" but can be
     evaluated in a "teleport-cluster" context. */}}
{{- define "teleport-cluster.auth.operatorFullName" -}}
{{- if .Values.operator.fullnameOverride }}
    {{- .Values.operator.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
    {{- if .Values.operator.nameOverride }}
        {{- printf "%s-%s" .Release.Name .Values.operator.nameOverride | trunc 63 | trimSuffix "-" }}
    {{- else }}
        {{- if contains "teleport-operator" .Release.Name }}
            {{- .Release.Name | trunc 63 | trimSuffix "-" }}
        {{- else }}
            {{- printf "%s-%s" .Release.Name "teleport-operator" | trunc 63 | trimSuffix "-" }}
        {{- end }}
    {{- end }}
{{- end -}}
{{- end -}}

{{/* Matches the operator template "teleport-cluster.operator.serviceAccountName"
     but can be evaluated in a "teleport-cluster" context. */}}
{{- define "teleport-cluster.auth.operatorServiceAccountName" -}}
{{- coalesce .Values.operator.serviceAccount.name (include "teleport-cluster.auth.operatorFullName" .) -}}
{{- end -}}
