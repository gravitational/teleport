{{/* Resolves the Teleport version: the explicit override at
.Values.teleportVersionOverride if set, otherwise the chart version. Pass the
root context (the helper mandates the standard `.Values.teleportVersionOverride`
location so callers don't have to assemble a context):
  include "teleport-util-lib.version" .
*/}}
{{- define "teleport-util-lib.version" -}}
{{- coalesce .Values.teleportVersionOverride .Chart.Version -}}
{{- end -}}

{{/* Major version number (e.g. "19") derived from teleport-util-lib.version.
Pass the root context, same as teleport-util-lib.version. */}}
{{- define "teleport-util-lib.majorVersion" -}}
{{- (semver (include "teleport-util-lib.version" .)).Major -}}
{{- end -}}
