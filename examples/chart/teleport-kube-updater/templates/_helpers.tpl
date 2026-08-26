{{- define "teleport-kube-updater.all" }}
---
{{ include "teleport-kube-updater.deployment" . }}
---
{{ include "teleport-kube-updater.role" . }}
---
{{ include "teleport-kube-updater.rolebinding" . }}
---
{{ include "teleport-kube-updater.serviceaccount" . }}
{{- end -}}