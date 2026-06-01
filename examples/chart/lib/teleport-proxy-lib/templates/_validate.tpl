{{- define "teleport-proxy-lib.internal.validate" -}}
{{- if not (and (hasKey .Values "clusterName") (kindIs "string" .Values.clusterName)) -}}
{{- fail "clusterName must be a string in teleport-proxy-lib values" -}}
{{- end -}}
{{- if not (and (hasKey .Values "teleportAuthService") (kindIs "string" .Values.teleportAuthService) (ne .Values.teleportAuthService "")) -}}
{{- fail "teleportAuthService must be a non-empty string in teleport-proxy-lib values" -}}
{{- end -}}
{{- /* Boolean inputs: must be a bool when set. */ -}}
{{- range $k := (list
  "acme"
  "disableTopologySpreadConstraints"
  "enterprise"
  "forceHAReplicas"
  "insecureSkipProxyTLSVerify"
  "separateMongoListener"
  "separatePostgresListener"
  "validateConfigOnDeploy"
) -}}
{{- if and (hasKey $.Values $k) (not (kindIs "bool" (index $.Values $k))) -}}
{{- fail (printf "%s must be a boolean in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* String inputs: must be a string when set. */ -}}
{{- range $k := (list
  "acmeEmail"
  "acmeURI"
  "chartMode"
  "enterpriseImage"
  "image"
  "imagePullPolicy"
  "logLevel"
  "nameOverride"
  "priorityClassName"
  "proxyListenerMode"
  "proxyProtocol"
  "teleportVersionOverride"
) -}}
{{- if and (hasKey $.Values $k) (not (kindIs "string" (index $.Values $k))) -}}
{{- fail (printf "%s must be a string in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* List inputs: must be a list when set. */ -}}
{{- range $k := (list
  "extraArgs"
  "extraContainers"
  "extraEnv"
  "extraVolumeMounts"
  "extraVolumes"
  "imagePullSecrets"
  "initContainers"
  "kubePublicAddr"
  "mongoPublicAddr"
  "mysqlPublicAddr"
  "postgresPublicAddr"
  "publicAddr"
  "sshPublicAddr"
  "tolerations"
  "topologySpreadConstraints"
  "tunnelPublicAddr"
) -}}
{{- if and (hasKey $.Values $k) (not (kindIs "slice" (index $.Values $k))) -}}
{{- fail (printf "%s must be a list in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* Map inputs: must be a map when set. */ -}}
{{- range $k := (list
  "affinity"
  "annotations"
  "extraLabels"
  "highAvailability"
  "ingress"
  "jobResources"
  "log"
  "nodeSelector"
  "podSecurityContext"
  "postStart"
  "readinessProbe"
  "resources"
  "securityContext"
  "service"
  "serviceAccount"
  "teleportConfig"
  "tls"
) -}}
{{- if and (hasKey $.Values $k) (not (kindIs "map" (index $.Values $k))) -}}
{{- fail (printf "%s must be a map in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* tls string sub-keys: must be strings when set. */ -}}
{{- $tls := $.Values.tls | default dict -}}
{{- range $k := (list "existingSecretName" "existingCASecretName" "existingCASecretKeyName") -}}
{{- if and (hasKey $tls $k) (not (kindIs "string" (index $tls $k))) -}}
{{- fail (printf "tls.%s must be a string in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* Numeric inputs: must be a number (int or float) when set. */ -}}
{{- range $k := (list "goMemLimitRatio" "probeTimeoutSeconds" "terminationGracePeriodSeconds") -}}
{{- if hasKey $.Values $k -}}
{{- $v := index $.Values $k -}}
{{- if not (or (kindIs "float64" $v) (kindIs "int" $v) (kindIs "int64" $v)) -}}
{{- fail (printf "%s must be a number in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- $ha := $.Values.highAvailability | default dict -}}
{{- range $k := (list "replicaCount" "minReadySeconds") -}}
{{- if hasKey $ha $k -}}
{{- $v := index $ha $k -}}
{{- if not (or (kindIs "float64" $v) (kindIs "int" $v) (kindIs "int64" $v)) -}}
{{- fail (printf "highAvailability.%s must be a number in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- $rp := $.Values.readinessProbe | default dict -}}
{{- range $k := (list "initialDelaySeconds" "periodSeconds" "failureThreshold" "successThreshold") -}}
{{- if hasKey $rp $k -}}
{{- $v := index $rp $k -}}
{{- if not (or (kindIs "float64" $v) (kindIs "int" $v) (kindIs "int64" $v)) -}}
{{- fail (printf "readinessProbe.%s must be a number in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- /* highAvailability nested sub-fields. */ -}}
{{- $ha := $.Values.highAvailability | default dict -}}
{{- if and (hasKey $ha "requireAntiAffinity") (not (kindIs "bool" $ha.requireAntiAffinity)) -}}
{{- fail "highAvailability.requireAntiAffinity must be a boolean in teleport-proxy-lib values" -}}
{{- end -}}
{{- $pdb := $ha.podDisruptionBudget | default dict -}}
{{- if and (hasKey $pdb "enabled") (not (kindIs "bool" $pdb.enabled)) -}}
{{- fail "highAvailability.podDisruptionBudget.enabled must be a boolean in teleport-proxy-lib values" -}}
{{- end -}}
{{- if hasKey $pdb "minAvailable" -}}
{{- $v := $pdb.minAvailable -}}
{{- if not (or (kindIs "float64" $v) (kindIs "int" $v) (kindIs "int64" $v) (kindIs "string" $v)) -}}
{{- fail "highAvailability.podDisruptionBudget.minAvailable must be a number or string (IntOrString) in teleport-proxy-lib values" -}}
{{- end -}}
{{- end -}}
{{- $cm := $ha.certManager | default dict -}}
{{- range $k := (list "enabled" "addCommonName" "addPublicAddrs") -}}
{{- if and (hasKey $cm $k) (not (kindIs "bool" (index $cm $k))) -}}
{{- fail (printf "highAvailability.certManager.%s must be a boolean in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- range $k := (list "issuerName" "issuerKind" "issuerGroup") -}}
{{- if and (hasKey $cm $k) (not (kindIs "string" (index $cm $k))) -}}
{{- fail (printf "highAvailability.certManager.%s must be a string in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- /* service nested sub-fields. */ -}}
{{- $svc := $.Values.service | default dict -}}
{{- if and (hasKey $svc "type") (not (kindIs "string" $svc.type)) -}}
{{- fail "service.type must be a string in teleport-proxy-lib values" -}}
{{- end -}}
{{- if and (hasKey $svc "spec") (not (kindIs "map" $svc.spec)) -}}
{{- fail "service.spec must be a map in teleport-proxy-lib values" -}}
{{- end -}}
{{- /* ingress nested sub-fields. */ -}}
{{- $ing := $.Values.ingress | default dict -}}
{{- range $k := (list "enabled" "useExisting" "suppressAutomaticWildcards") -}}
{{- if and (hasKey $ing $k) (not (kindIs "bool" (index $ing $k))) -}}
{{- fail (printf "ingress.%s must be a boolean in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- if and (hasKey $ing "spec") (not (kindIs "map" $ing.spec)) -}}
{{- fail "ingress.spec must be a map in teleport-proxy-lib values" -}}
{{- end -}}
{{- /* serviceAccount nested sub-fields. */ -}}
{{- $sa := $.Values.serviceAccount | default dict -}}
{{- if and (hasKey $sa "create") (not (kindIs "bool" $sa.create)) -}}
{{- fail "serviceAccount.create must be a boolean in teleport-proxy-lib values" -}}
{{- end -}}
{{- if and (hasKey $sa "name") (not (kindIs "string" $sa.name)) -}}
{{- fail "serviceAccount.name must be a string in teleport-proxy-lib values" -}}
{{- end -}}
{{- /* log nested sub-fields. */ -}}
{{- $log := $.Values.log | default dict -}}
{{- range $k := (list "level" "output" "format") -}}
{{- if and (hasKey $log $k) (not (kindIs "string" (index $log $k))) -}}
{{- fail (printf "log.%s must be a string in teleport-proxy-lib values" $k) -}}
{{- end -}}
{{- end -}}
{{- if and (hasKey $log "extraFields") (not (kindIs "slice" $log.extraFields)) -}}
{{- fail "log.extraFields must be a list in teleport-proxy-lib values" -}}
{{- end -}}
{{- /* postStart nested sub-fields. */ -}}
{{- $ps := $.Values.postStart | default dict -}}
{{- if and (hasKey $ps "command") (not (kindIs "slice" $ps.command)) -}}
{{- fail "postStart.command must be a list in teleport-proxy-lib values" -}}
{{- end -}}
{{- end -}}
