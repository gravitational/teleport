{{- define "teleport-proxy-lib.internal.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name (printf "%s-proxy" .Release.Name) -}}
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
{{- define "teleport-proxy-lib.internal.hookServiceAccountName" -}}
{{- include "teleport-proxy-lib.internal.serviceAccountName" . -}}
{{- if .Values.serviceAccount.create -}}
-hook
{{- end -}}
{{- end -}}

{{/* Proxy all labels */}}
{{- define "teleport-proxy-lib.internal.labels" -}}
{{ include "teleport-proxy-lib.internal.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-util-lib.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-util-lib.majorVersion" . }}'
{{- end -}}

{{/* Proxy selector labels */}}
{{- define "teleport-proxy-lib.internal.selectorLabels" -}}
app.kubernetes.io/name: '{{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'proxy'
{{- end -}}
