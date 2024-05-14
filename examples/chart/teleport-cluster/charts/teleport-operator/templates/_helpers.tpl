{{/*
Expand the name of the chart.
*/}}
{{- define "teleport-cluster.operator.name" -}}
    {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
This is a modified version of the default fully qualified app name helper.
We diverge by always honouring "nameOverride" when it's set, as opposed to the
default behaviour of shortening if `nameOverride` is included in chart name.
This is done to avoid naming conflicts when including th chart in `teleport-cluster`
*/}}
{{- define "teleport-cluster.operator.fullname" -}}
    {{- if .Values.fullnameOverride }}
        {{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
    {{- else }}
        {{- if .Values.nameOverride }}
            {{- printf "%s-%s" .Release.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
        {{- else }}
            {{- if contains .Chart.Name .Release.Name }}
                {{- .Release.Name | trunc 63 | trimSuffix "-" }}
            {{- else }}
                {{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
            {{- end }}
        {{- end }}
    {{- end }}
{{- end }}

{{/*
Create the name of the service account to use
if serviceAccount is not defined or serviceAccount.name is empty, use .Release.Name
*/}}
{{- define "teleport-cluster.operator.serviceAccountName" -}}
{{- coalesce .Values.serviceAccount.name (include "teleport-cluster.operator.fullname" .) -}}
{{- end -}}

{{- define "teleport-cluster.version" -}}
{{- coalesce .Values.teleportVersionOverride .Chart.Version }}
{{- end -}}

{{- define "teleport-cluster.majorVersion" -}}
{{- (semver (include "teleport-cluster.version" .)).Major -}}
{{- end -}}

{{/* Operator selector labels */}}
{{- define "teleport-cluster.operator.selectorLabels" -}}
app.kubernetes.io/name: '{{ include "teleport-cluster.operator.name" . }}'
app.kubernetes.io/instance: '{{ .Release.Name }}'
app.kubernetes.io/component: 'operator'
{{- end -}}

{{/* Operator all labels */}}
{{- define "teleport-cluster.operator.labels" -}}
{{ include "teleport-cluster.operator.selectorLabels" . }}
helm.sh/chart: '{{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}'
app.kubernetes.io/managed-by: '{{ .Release.Service }}'
app.kubernetes.io/version: '{{ include "teleport-cluster.version" . }}'
teleport.dev/majorVersion: '{{ include "teleport-cluster.majorVersion" . }}'
teleport.dev/release: '{{ include "teleport-cluster.operator.namespacedRelease" . }}'
{{- end -}}

{{/* Teleport auth or proxy address */}}
{{- define "teleport-cluster.operator.teleportAddress" -}}
{{- $clusterAddr := include "teleport-cluster.auth.serviceFQDN" . -}}
{{- if empty $clusterAddr -}}
    {{- required "The `teleportAddress` value is mandatory when deploying a standalone operator." .Values.teleportAddress -}}
    {{- if and (eq .Values.joinMethod "kubernetes") (empty .Values.teleportClusterName) (not (hasSuffix ":3025" .Values.teleportAddress)) -}}
        {{- fail "When joining using the Kubernetes JWKS join method, you must set the value `teleportClusterName`" -}}
    {{- end -}}
{{- else -}}
    {{- $clusterAddr | printf "%s:3025" -}}
{{- end -}}
{{- end -}}

{{- /* This template is a placeholder.
If we are imported by the main chart "teleport-cluster" it is overridden*/ -}}
{{- define "teleport-cluster.auth.serviceFQDN" -}}{{- end }}

{{- /* This templates returns "true" or "false"  describing if the CRDs should be deployed.
If we have an explicit requirement ("always" or "never") things are easy.
If we don't we check if the operator is enabled.
However, we cannot just trash the CRDs if the operator is disabled, this causes
a mass CR deletion and users will shoot themselves in the foot whith this
(temporarily disabling the operator would cause havoc).
So we check if there's a CRD already deployed, it that's the case, we keep the CRDs.
*/ -}}
{{- define "teleport-cluster.operator.shouldInstallCRDs" -}}
  {{- if eq .Values.installCRDs "always" -}}
    true
  {{- else if eq .Values.installCRDs "never" -}}
    false
  {{- else if eq .Values.installCRDs "dynamic" -}}
    {{- if .Values.enabled -}}
      true
    {{- else -}}
      {{- include "teleport-cluster.operator.checkExistingCRDs" . -}}
    {{- end -}}
  {{- else -}}
    {{- fail ".Values.installCRDs must be 'never', 'always' or 'dynamic'." -}}
  {{- end -}}
{{- end -}}

{{- /* This template checks if a known CRD is depployed (rolev7) and owned by
the release. As CRDs are not namespaced, we must use a custom annotation to avoid
a conflict when two releases are deployed with the same name in different namespaces. */ -}}
{{- define "teleport-cluster.operator.checkExistingCRDs" -}}
  {{ $existingCRD := lookup "apiextensions.k8s.io/v1" "CustomResourceDefinition" "" "teleportrolesv7.resources.teleport.dev"}}
  {{- if not $existingCRD -}}
    false
  {{- else -}}
    {{- $release := index $existingCRD.metadata.labels "teleport.dev/release" }}
    {{- if eq $release (include "teleport-cluster.operator.namespacedRelease" .) -}}
      true
    {{- else -}}
    false
    {{- end -}}
  {{- end -}}
{{- end -}}

{{- /* This is a custom label containing the namespaced release.
This is used to avoid conflicts for non-namespaced resources like CRDs. */ -}}
{{- define "teleport-cluster.operator.namespacedRelease" -}}
  {{ .Release.Namespace }}_{{ .Release.Name }}
{{- end -}}

{{- /* This is the object merged with CRDs manifests to enrich them (add labels). */ -}}
{{- define "teleport-cluster.operator.crdOverrides" -}}
metadata:
  labels: {{- include "teleport-cluster.operator.labels" . | nindent 4 }}
{{- end -}}
