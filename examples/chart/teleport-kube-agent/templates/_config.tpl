{{- define "teleport-kube-agent.config" -}}
{{- $logLevel := (coalesce .Values.logLevel .Values.log.level "INFO") -}}
{{- $appRolePresent := contains "app" (.Values.roles | toString) -}}
{{- $discoveryEnabled := contains "discovery" (.Values.roles | toString) -}}
{{- $appDiscoveryEnabled := and ($appRolePresent) ($discoveryEnabled) -}}
{{- if (ge (include "teleport-kube-agent.version" . | semver).Major 11) }}
version: v3
{{- end }}
teleport:
  join_params:
    method: "{{ .Values.joinParams.method }}"
    token_name: "/etc/teleport-secrets/auth-token"
  {{- if (ge (include "teleport-kube-agent.version" . | semver).Major 11) }}
  proxy_server: {{ required "proxyAddr is required in chart values" .Values.proxyAddr }}
  {{- else }}
  auth_servers: ["{{ required "proxyAddr is required in chart values" .Values.proxyAddr }}"]
  {{- end }}
  {{- if .Values.caPin }}
  ca_pin: {{- toYaml .Values.caPin | nindent 4 }}
  {{- end }}
  log:
    severity: {{ $logLevel }}
    output: {{ .Values.log.output }}
    format:
      output: {{ .Values.log.format }}
      extra_fields: {{ .Values.log.extraFields | toJson }}

kubernetes_service:
  {{- if or (contains "kube" (.Values.roles | toString)) (empty .Values.roles) }}
  enabled: true
  kube_cluster_name: {{ required "kubeClusterName is required in chart values when kube role is enabled, see README" .Values.kubeClusterName }}
    {{- if .Values.labels }}
  labels: {{- toYaml .Values.labels | nindent 4 }}
    {{- end }}
  {{- else }}
  enabled: false
  {{- end }}

{{- if and (or (.Values.apps) (.Values.appResources)) (not ($appRolePresent)) }}
  {{- fail "app role should be enabled if one of 'apps' or 'appResources' is set, see README" }}
{{- end }}

app_service:
  {{- if $appRolePresent }}
    {{- if not (or (.Values.apps) (.Values.appResources) ($appDiscoveryEnabled)) }}
      {{- fail "app service is enabled, but no application source is enabled. You must either statically define apps through `apps`, dynamically through `appResources`, or enable in-cluster discovery." }}
    {{- end }}
  enabled: true
  {{- if .Values.apps }}
    {{- range $app := .Values.apps }}
      {{- if not (hasKey $app "name") }}
        {{- fail "'name' is required for all 'apps' in chart values when app role is enabled, see README" }}
      {{- end }}
      {{- if not (hasKey $app "uri") }}
        {{- fail "'uri' is required for all 'apps' in chart values when app role is enabled, see README" }}
      {{- end }}
    {{- end }}
  apps:
    {{- toYaml .Values.apps | nindent 4 }}
    {{- end }}
  resources:
    {{- if .Values.appResources }}
      {{- toYaml .Values.appResources | nindent 4 }}
    {{- end }}
    {{- if $appDiscoveryEnabled }}
    - labels:
        "teleport.dev/kubernetes-cluster": "{{ required "kubeClusterName is required in chart values when kube or discovery role is enabled, see README" .Values.kubeClusterName }}"
        "teleport.dev/origin": "discovery-kubernetes"
    {{- end }}
  {{- else }}
  enabled: false
  {{- end }}

db_service:
  {{- if contains "db" (.Values.roles | toString) }}
  enabled: true
  {{- if not (or (.Values.awsDatabases) (.Values.azureDatabases) (.Values.databases) (.Values.databaseResources)) }}
    {{- fail "at least one of 'awsDatabases', 'azureDatabases', 'databases' or 'databaseResources' is required in chart values when db role is enabled, see README" }}
  {{- end }}
  {{- if .Values.awsDatabases }}
  aws:
    {{- range $awsDb := .Values.awsDatabases }}
      {{- if not (hasKey $awsDb "types") }}
        {{- fail "'types' is required for all 'awsDatabases' in chart values when key is set and db role is enabled, see README" }}
      {{- end }}
      {{- if not (hasKey $awsDb "regions") }}
        {{- fail "'regions' is required for all 'awsDatabases' in chart values when key is set and db role is enabled, see README" }}
      {{- end }}
      {{- if not (hasKey $awsDb "tags") }}
        {{- fail "'tags' is required for all 'awsDatabases' in chart values when key is set and db role is enabled, see README" }}
      {{- end }}
    {{- end }}
    {{- toYaml .Values.awsDatabases | nindent 4 }}
  {{- end }}
  {{- if .Values.azureDatabases }}
  azure:
    {{- toYaml .Values.azureDatabases | nindent 4 }}
  {{- end}}
  {{- if .Values.databases }}
  databases:
    {{- range $db := .Values.databases }}
      {{- if not (hasKey $db "name") }}
        {{- fail "'name' is required for all 'databases' in chart values when db role is enabled, see README" }}
      {{- end }}
      {{- if not (hasKey $db "uri") }}
        {{- fail "'uri' is required for all 'databases' is required in chart values when db role is enabled, see README" }}
      {{- end }}
      {{- if not (hasKey $db "protocol") }}
        {{- fail "'protocol' is required for all 'databases' in chart values when db role is enabled, see README" }}
      {{- end }}
    {{- end }}
    {{- toYaml .Values.databases | nindent 4 }}
  {{- end }}
  {{- if .Values.databaseResources }}
  resources:
    {{- toYaml .Values.databaseResources | nindent 4 }}
  {{- end }}
{{- else }}
  enabled: false
{{- end }}

discovery_service:
{{- if $discoveryEnabled }}
  enabled: true
  discovery_group: {{ required "kubeClusterName is required in chart values when kube or discovery role is enabled, see README" .Values.kubeClusterName }}
  kubernetes: {{- toYaml .Values.kubernetesDiscovery | nindent 4 }}
{{- else }}
  enabled: false
{{- end }}

jamf_service:
  {{- if contains "jamf" (.Values.roles | toString) }}
  enabled: true
  api_endpoint: {{ required "jamfApiEndpoint is required in chart values when jamf role is enabled, see README" .Values.jamfApiEndpoint }}
  client_id: {{ required "jamfClientId is required in chart values when jamf role is enabled, see README" .Values.jamfClientId }}
  client_secret_file: "/etc/teleport-jamf-api-credentials/credential"
  {{- else }}
  enabled: false
  {{- end }}

auth_service:
  enabled: false
ssh_service:
  enabled: false
proxy_service:
  enabled: false
{{- end -}}
