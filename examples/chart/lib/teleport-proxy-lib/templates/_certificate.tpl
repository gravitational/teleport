{{- define "teleport-proxy-lib.internal.certificate" }}
{{- if .Values.highAvailability.certManager.enabled -}}
  {{- /* Append clusterName and wildcard version to list of dnsNames on certificate request (original functionality) */ -}}
  {{- $domainList := list (required "clusterName is required in chartValues when certManager is enabled" .Values.clusterName) -}}
  {{- $domainList := append $domainList (printf "*.%s" (required "clusterName is required in chartValues when certManager is enabled" .Values.clusterName)) -}}
  {{- /* If the config option is enabled and at least one publicAddr is set, append all public addresses to the list of dnsNames */ -}}
  {{- if and .Values.highAvailability.certManager.addPublicAddrs (gt (len .Values.publicAddr) 0) -}}
    {{- /* Trim ports from all public addresses if present */ -}}
    {{- range .Values.publicAddr -}}
      {{- $address := . -}}
      {{- if (contains ":" $address) -}}
        {{- $split := split ":" $address -}}
        {{- $address = $split._0 -}}
      {{- end -}}
      {{- $domainList = append (mustWithout $domainList .) $address -}}
    {{- end -}}
  {{- end -}}
  {{- /* Finally, remove any duplicate entries from the list of domains */ -}}
  {{- $domainList := mustUniq $domainList -}}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
spec:
  secretName: teleport-tls
  {{- if .Values.highAvailability.certManager.addCommonName }}
  commonName: {{ quote .Values.clusterName }}
  {{- end }}
  dnsNames:
  {{- range $domainList }}
  - {{ quote . }}
  {{- end }}
  issuerRef:
    name: {{ required "highAvailability.certManager.issuerName is required in chart values" .Values.highAvailability.certManager.issuerName }}
    kind: {{ required "highAvailability.certManager.issuerKind is required in chart values" .Values.highAvailability.certManager.issuerKind }}
    group: {{ required "highAvailability.certManager.issuerGroup is required in chart values" .Values.highAvailability.certManager.issuerGroup }}
  {{- if or .Values.annotations.certSecret .Values.extraLabels.certSecret }}
  secretTemplate:
    {{- with .Values.annotations.certSecret }}
    annotations: {{- toYaml . | nindent 6 }}
    {{- end }}
    {{- with .Values.extraLabels.certSecret }}
    labels: {{- toYaml . | nindent 6 }}
    {{- end }}
  {{- end }}
{{- end }}
{{- end }}{{/* certificate */}}
