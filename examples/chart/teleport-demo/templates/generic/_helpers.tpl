{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "teleport.name" -}}
{{- default .Release.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "teleport.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Release.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "teleport.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "teleport.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "teleport.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*  Manage the labels for each entity  */}}
{{- define "teleport.labels" -}}
app: {{ template "teleport.fullname" . }}
fullname: {{ template "teleport.fullname" . }}
chart: {{ template "teleport.chart" . }}
release: {{ .Release.Name }}
heritage: {{ .Release.Service }}
{{- end -}}

{{/* Public authssh address should be set according to ServiceType */}}
{{- define "teleport.main.authssh_public_addr" -}}
{{ $.clusterName }}
{{- if contains "LoadBalancer" $.Values.service.type -}}
{{ template "teleport.fullname" $ }}-{{ $.Values.mainClusterName }}.{{ $.Values.cloudflare.domain }}:{{ $.Values.service.ports.authssh.port }}
{{- else -}}
{{ $.Values.minikubeIP }}:{{ $.Values.nodePort.ports.authssh.nodePort }}
{{- end -}}
{{- end -}}

{{/* Public proxyweb address should be set according to ServiceType */}}
{{- define "teleport.main.proxyweb_public_addr" -}}
{{- if contains "LoadBalancer" .Values.service.type -}}
{{ template "teleport.fullname" . }}-{{ .Values.mainClusterName }}.{{ .Values.cloudflare.domain }}:{{ .Values.service.ports.proxyweb.port }}
{{- else -}}
{{ .Values.minikubeIP }}:{{ .Values.nodePort.ports.proxyweb.nodePort }}
{{- end -}}
{{- end -}}

{{/* Public proxykube address should be set according to ServiceType */}}
{{- define "teleport.main.proxykube_public_addr" -}}
{{- if contains "LoadBalancer" .Values.service.type -}}
{{ template "teleport.fullname" . }}-{{ .Values.mainClusterName }}.{{ .Values.cloudflare.domain }}:{{ .Values.service.ports.proxykube.port }}
{{- else -}}
{{ .Values.minikubeIP }}:{{ .Values.nodePort.ports.proxykube.nodePort }}
{{- end -}}
{{- end -}}

{{/* Public proxyssh address should be set according to ServiceType */}}
{{- define "teleport.main.proxyssh_public_addr" -}}
{{- if contains "LoadBalancer" .Values.service.type -}}
{{ template "teleport.fullname" . }}-{{ .Values.mainClusterName }}.{{ .Values.cloudflare.domain }}:{{ .Values.service.ports.proxyssh.port }}
{{- else -}}
{{ .Values.minikubeIP }}:{{ .Values.nodePort.ports.proxyssh.nodePort }}
{{- end -}}
{{- end -}}

{{/* Public proxytunnel address should be set according to ServiceType */}}
{{- define "teleport.main.proxytunnel_public_addr" -}}
{{- if contains "LoadBalancer" .Values.service.type -}}
{{ template "teleport.fullname" . }}-{{ .Values.mainClusterName }}.{{ .Values.cloudflare.domain }}:{{ .Values.service.ports.proxytunnel.port }}
{{- else -}}
{{ .Values.minikubeIP }}:{{ .Values.nodePort.ports.proxytunnel.nodePort }}
{{- end -}}
{{- end -}}