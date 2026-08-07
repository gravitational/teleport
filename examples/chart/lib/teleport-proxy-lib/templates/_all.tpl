{{- define "teleport-proxy-lib.all" -}}
{{- include "teleport-proxy-lib.internal.validate" . -}}
{{- $root := . -}}
{{/*
  Render each manifest template, but only emit a document (and its `---`
  separator) for templates that actually produce output. Optional templates
  (certificate, ingress, pdb, ...) render nothing when disabled; emitting an
  unconditional `---` for them would leave empty YAML documents that shift
  helm-unittest documentIndex values depending on which features are enabled.
*/}}
{{- $docs := list -}}
{{- range $tpl := (list
  "teleport-proxy-lib.internal.certificate"
  "teleport-proxy-lib.internal.config"
  "teleport-proxy-lib.internal.deployment"
  "teleport-proxy-lib.internal.ingress"
  "teleport-proxy-lib.internal.pdb"
  "teleport-proxy-lib.internal.predeploy_config"
  "teleport-proxy-lib.internal.predeploy_job"
  "teleport-proxy-lib.internal.predeploy_serviceaccount"
  "teleport-proxy-lib.internal.service"
  "teleport-proxy-lib.internal.serviceaccount"
) -}}
{{- $out := trim (include $tpl $root) -}}
{{- if $out -}}
{{- $docs = append $docs $out -}}
{{- end -}}
{{- end -}}
{{ $docs | join "\n---\n" }}
{{- end -}}
