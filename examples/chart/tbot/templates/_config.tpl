{{ if not (or .Values.teleportAuthAddress .Values.teleportProxyAddress) }}
  {{- $_ := required "`teleportAuthAddress` or `teleportProxyAddress` must be provided" "" }}
{{ end }}
{{ if not .Values.clusterName }}
  {{- $_ := required "`clusterName` must be provided" "" }}
{{ end }}
{{ if not .Values.token }}
  {{- $_ := required "`token` must be provided" "" }}
{{ end }}
{{ if (and .Values.teleportAuthAddress .Values.teleportProxyAddress) }}
  {{- $_ := required "`teleportAuthAddress` and `teleportProxyAddress` are mutually exclusive" "" }}
{{ end }}
{{- define "tbot.config" -}}
version: v2
{{- if .Values.teleportProxyAddress }}
proxy_server: {{ .Values.teleportProxyAddress }}
{{- end }}
{{- if .Values.teleportAuthAddress }}
auth_server: {{ .Values.teleportAuthAddress }}
{{- end }}
onboarding:
  join_method: {{ .Values.joinMethod }}
  token: {{ .Values.token }}
{{- if eq .Values.persistence "disabled" }}
storage:
  type: memory
{{- else if eq .Values.persistence "secret" }}
storage:
  type: kubernetes_secret
  name: {{ include "tbot.fullname" . }}
{{- else }}
  {{- required "'persistence' must be 'secret' or 'disabled'" "" }}
{{- end }}
{{- if or (.Values.defaultOutput.enabled) (.Values.argocd.enabled) (.Values.outputs) }}
outputs:
{{- if .Values.defaultOutput.enabled }}
  - type: identity
    destination:
      type: kubernetes_secret
      name: {{ include "tbot.defaultOutputName" . }}
{{- end }}
{{- if .Values.argocd.enabled }}
  - type: kubernetes/argo-cd
    {{- if .Values.argocd.clusterSelectors }}
    selectors:
        {{- toYaml .Values.argocd.clusterSelectors | nindent 8 }}
    {{- else }}
        {{- required "'argocd.clusterSelectors' must be provided if `argocd.enabled' is true" "" }}
    {{- end }}
    {{- if .Values.argocd.secretNamespace }}
    secret_namespace: {{ .Values.argocd.secretNamespace }}
    {{- end }}
    {{- if .Values.argocd.secretLabels }}
    secret_labels:
        {{- toYaml .Values.argocd.secretLabels | nindent 8 }}
    {{- end }}
    {{- if .Values.argocd.secretAnnotations }}
    secret_annotations:
        {{- toYaml .Values.argocd.secretAnnotations | nindent 8 }}
    {{- end }}
    {{- if .Values.argocd.project }}
    project: {{ .Values.argocd.project }}
    {{- end }}
    {{- if .Values.argocd.namespaces }}
    namespaces:
        {{- toYaml .Values.argocd.namespaces | nindent 8 }}
    {{- end }}
    {{- if .Values.argocd.clusterResources }}
    cluster_resources: {{ .Values.argocd.clusterResources }}
    {{- end }}
{{- end }}
{{- if .Values.outputs }}
{{- toYaml .Values.outputs | nindent 2}}
{{- end }}
{{- end }}
{{- if .Values.services }}
services: {{- toYaml .Values.services | nindent 2}}
{{- end }}
{{- end -}}
