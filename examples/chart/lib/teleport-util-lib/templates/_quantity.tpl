{{/* This template tries to parse a resource quantity like Kubernetes does.
Helm sadly doesn't offer this critical primitive: https://github.com/helm/helm/issues/11376
The quantity serialization format is described here: https://github.com/kubernetes/apimachinery/blob/master/pkg/api/resource/quantity.go#L33

This template support IEC, SI and decimal notation syntaxes, but has poor error handling.*/}}
{{- define "teleport-util-lib.resource-quantity" -}}
    {{- $value := . -}}
    {{- $unit := 1.0 -}}
    {{- if typeIs "string" . -}}
        {{- $base2 := dict "Ki" 0x1p10 "Mi" 0x1p20 "Gi" 0x1p30 "Ti" 0x1p40 "Pi" 0x1p50 "Ei" 0x1p60 -}}
        {{- $base10 := dict "m" 1e-3 "k" 1e3 "M" 1e6 "G" 1e9 "T" 1e12 "P" 1e15 "E" 1e18 -}}
        {{- range $k, $v := merge $base2 $base10 -}}
            {{- if hasSuffix $k $ -}}
                {{- $value = trimSuffix $k $ -}}
                {{- $unit = $v -}}
            {{- end -}}
        {{- end -}}
    {{- end -}}
    {{- mulf (float64 $value) $unit -}}
{{- end -}}

{{/* This renders the GOMEMLIMIT env var unless the user already specified it
in extraEnv, goMemLimitRatio is set to 0, or requests.memory.limit is unset.

Important: This template should be called on a context with the following keys:
- .resources - A standard k8s container resources definition
- .extraEnv - A dict potentially containing a "GOMEMLIMIT" key, which, if
   defined, will be used instead of calculating the memory limit from the k8s
   resource limit.
- .goMemLimitRatio - The ratio of the go memory limit to the k8s memory limit

*/}}
{{- define "teleport-util-lib.gomemlimit" -}}
    {{- $alreadySet := false -}}
    {{- range $_, $var := .extraEnv -}}
        {{- if eq $var.name "GOMEMLIMIT" -}}
            {{- $alreadySet = true -}}
        {{- end -}}
    {{- end -}}
    {{- if and (not $alreadySet) .goMemLimitRatio -}}
        {{- $ratio := .goMemLimitRatio -}}
        {{- with .resources }}{{ with .limits }}{{ with .memory -}}
            {{- include "teleport-util-lib.resource-quantity" . | float64 | mulf $ratio | ceil | int -}}
        {{- end }}{{ end }}{{ end -}}
    {{- end -}}
{{- end -}}
