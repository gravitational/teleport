{{- define "teleport.kube.agent.isUpgrade" -}}
{{- /* Checks if action is an upgrade from an old release that didn't supported Secret storage */}}
{{- /* TODO: change the behaviour for Teleport 11 */}}
{{- if and .Release.IsUpgrade  }}
  {{- $deployment := (lookup "v1" "Deployment" .Release.Namespace .Release.Name) -}}
  {{- $statefulset := (lookup "v1" "Statefulset" .Release.Namespace .Release.Name) -}}
  {{- if ($deployment) }}
true
  {{- else if and $statefulset $statefulset.spec }}
    {{- if ($statefulset.spec.volumeClaimTemplates) }}
true
   {{- end }}
  {{- else if .Values.unitTestUpgrade }}
true
  {{- end }}
{{- end }}
{{- end -}}