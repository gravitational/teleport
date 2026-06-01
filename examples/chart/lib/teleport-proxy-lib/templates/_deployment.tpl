{{- define "teleport-proxy-lib.internal.deployment" }}
{{- $proxy := (mustDeepCopy .Values) -}}
{{- $replicable := or $proxy.highAvailability.certManager.enabled $proxy.tls.existingSecretName $proxy.ingress.enabled -}}
{{- $projectedServiceAccountToken := semverCompare ">=1.20.0-0" .Capabilities.KubeVersion.Version }}
{{- $topologySpreadConstraints := and (semverCompare ">=1.18.0-0" .Capabilities.KubeVersion.Version) (not $proxy.disableTopologySpreadConstraints) }}
# Deployment is {{ if not $replicable }}not {{end}}replicable
{{- if and $proxy.highAvailability.certManager.enabled $proxy.tls.existingSecretName }}
{{- fail "Cannot set both highAvailability.certManager.enabled and tls.existingSecretName, choose one or the other" }}
{{- end }}
{{- if and $proxy.acme $proxy.tls.existingSecretName }}
{{- fail "Cannot set both acme.enabled and tls.existingSecretName, choose one or the other" }}
{{- end }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-proxy
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if $proxy.extraLabels.deployment }}
    {{- toYaml $proxy.extraLabels.deployment | nindent 4 }}
    {{- end }}
{{- if $proxy.annotations.deployment }}
  annotations: {{- toYaml $proxy.annotations.deployment | nindent 4 }}
{{- end }}
spec:
{{- /*
  If proxies cannot be replicated we use a single replica.
  By default we want to upgrade all users to at least 2 replicas, if they had a higher replica count we take it.
  Users who want to force a single proxy can use forceHAReplicas.
*/}}
{{- if $replicable }}
  {{- if $proxy.forceHAReplicas }}
  replicas: {{ $proxy.highAvailability.replicaCount }}
  {{- else }}
  replicas: {{ max $proxy.highAvailability.replicaCount 2 }}
  {{- end }}
  {{- if $proxy.highAvailability.minReadySeconds }}
  minReadySeconds: {{ $proxy.highAvailability.minReadySeconds }}
  {{- end }}
{{- else }}
  replicas: 1
{{- end }}
  selector:
    matchLabels: {{- include "teleport-proxy-lib.internal.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        # ConfigMap checksum, to recreate the pod on config changes.
        checksum/config: {{ include "teleport-proxy-lib.internal.config" . | sha256sum }}
{{- if $proxy.annotations.pod }}
  {{- toYaml $proxy.annotations.pod | nindent 8 }}
{{- end }}
      labels:
        {{- include "teleport-proxy-lib.internal.labels" . | nindent 8 }}
        {{- if $proxy.extraLabels.pod }}
        {{- toYaml $proxy.extraLabels.pod | nindent 8 }}
        {{- end }}
    spec:
{{- if $proxy.nodeSelector }}
      nodeSelector: {{- toYaml $proxy.nodeSelector | nindent 8 }}
{{- end }}
{{- if $topologySpreadConstraints }}
  {{- if $proxy.topologySpreadConstraints }}
      topologySpreadConstraints: {{- toYaml $proxy.topologySpreadConstraints | nindent 8 }}
  {{- else }}
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels: {{- include "teleport-proxy-lib.internal.selectorLabels" . | nindent 14 }}
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels: {{- include "teleport-proxy-lib.internal.selectorLabels" . | nindent 14 }}
  {{- end }}
{{- end }}
      affinity:
{{- if $proxy.affinity }}
  {{- if $proxy.highAvailability.requireAntiAffinity }}
    {{- fail "Cannot use highAvailability.requireAntiAffinity when affinity is also set in chart values - unset one or the other" }}
  {{- end }}
  {{- toYaml $proxy.affinity | nindent 8 }}
{{- else }}
        podAntiAffinity:
  {{- if $proxy.highAvailability.requireAntiAffinity }}
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app.kubernetes.io/instance
                operator: In
                values:
                  - {{ .Release.Name }}
              - key: app.kubernetes.io/component
                operator: In
                values:
                - proxy
            topologyKey: "kubernetes.io/hostname"
  {{- else if gt (int $proxy.highAvailability.replicaCount) 1 }}
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 50
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app.kubernetes.io/instance
                  operator: In
                  values:
                    - {{ .Release.Name }}
                - key: app.kubernetes.io/component
                  operator: In
                  values:
                    - proxy
              topologyKey: "kubernetes.io/hostname"
  {{- end }}
{{- end }}
{{- if $proxy.tolerations }}
      tolerations: {{- toYaml $proxy.tolerations | nindent 6 }}
{{- end }}
{{- if $proxy.imagePullSecrets }}
      imagePullSecrets:
  {{- toYaml $proxy.imagePullSecrets | nindent 6 }}
{{- end }}
{{- if $proxy.initContainers }}
      initContainers:
  {{- range $initContainer := $proxy.initContainers }}
    {{- if and (not $initContainer.resources) $proxy.resources }}
      {{- $_ := set $initContainer "resources" $proxy.resources }}
    {{- end }}
    {{- $skipDefaultVolumeMounts := $initContainer.skipDefaultVolumeMounts | default false }}
    {{- list (omit $initContainer "skipDefaultVolumeMounts") | toYaml | nindent 8 }}
    {{- if not $skipDefaultVolumeMounts }}
          volumeMounts:
    {{- if or $proxy.highAvailability.certManager.enabled $proxy.tls.existingSecretName }}
          - mountPath: /etc/teleport-tls
            name: "teleport-tls"
            readOnly: true
    {{- end }}{{/* HA or existingSecretName */}}
          - mountPath: /etc/teleport
            name: "config"
            readOnly: true
          - mountPath: /var/lib/teleport
            name: "data"
    {{- if $proxy.extraVolumeMounts }}
      {{- toYaml $proxy.extraVolumeMounts | nindent 10 }}
    {{- end }}{{/* extraVolumeMounts */}}
    {{- end }}
  {{- end }}{{/* range $proxy.initContainers */}}
{{- end }}{{/* initContainers */}}
      containers:
      - name: "teleport"
        image: '{{ if $proxy.enterprise }}{{ $proxy.enterpriseImage }}{{ else }}{{ $proxy.image }}{{ end }}:{{ include "teleport-proxy-lib.internal.version" . }}'
        imagePullPolicy: {{ $proxy.imagePullPolicy }}
        {{- $gomemlimit := include "teleport-util-lib.gomemlimit" $proxy }}
        {{- if or $proxy.extraEnv $proxy.tls.existingCASecretName $gomemlimit }}
        env:
        {{- if $gomemlimit }}
        - name: GOMEMLIMIT
          value: {{ $gomemlimit | quote }}
        {{- end }}
        {{- if (gt (len $proxy.extraEnv) 0) }}
          {{- toYaml $proxy.extraEnv | nindent 8 }}
        {{- end }}
        {{- if $proxy.tls.existingCASecretName }}
        - name: SSL_CERT_FILE
          value: "/etc/teleport-tls-ca/{{ required "tls.existingCASecretKeyName must be set if tls.existingCASecretName is set in chart values" $proxy.tls.existingCASecretKeyName }}"
        {{- end }}
        {{- end }}
        args:
        - "--diag-addr=0.0.0.0:3000"
        {{- if $proxy.insecureSkipProxyTLSVerify }}
        - "--insecure"
        {{- end }}
        {{- if $proxy.extraArgs }}
          {{- toYaml $proxy.extraArgs | nindent 8 }}
        {{- end }}
        ports:
        - name: tls
          containerPort: 3080
          protocol: TCP
        {{- if $proxy.enterprise }}
        - name: proxypeering
          containerPort: 3021
          protocol: TCP
        {{- end }}
        {{- if ne $proxy.proxyListenerMode "multiplex" }}
        - name: sshproxy
          containerPort: 3023
          protocol: TCP
        - name: sshtun
          containerPort: 3024
          protocol: TCP
        - name: kube
          containerPort: 3026
          protocol: TCP
        - name: mysql
          containerPort: 3036
          protocol: TCP
        {{- if $proxy.separatePostgresListener }}
        - name: postgres
          containerPort: 5432
          protocol: TCP
        {{- end }}
        {{- if $proxy.separateMongoListener }}
        - name: mongo
          containerPort: 27017
          protocol: TCP
        {{- end }}
        {{- end }}
        - name: diag
          containerPort: 3000
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: diag
          initialDelaySeconds: 5 # wait 5s for agent to start
          periodSeconds: 5 # poll health every 5s
          failureThreshold: 6 # consider agent unhealthy after 30s (6 * 5s)
          timeoutSeconds: {{ $proxy.probeTimeoutSeconds }}
        readinessProbe:
          httpGet:
            path: /readyz
            port: diag
          initialDelaySeconds: {{ $proxy.readinessProbe.initialDelaySeconds }}
          periodSeconds: {{ $proxy.readinessProbe.periodSeconds }}
          failureThreshold: {{$proxy.readinessProbe.failureThreshold}}
          successThreshold: {{$proxy.readinessProbe.successThreshold}}
          timeoutSeconds: {{ $proxy.probeTimeoutSeconds }}
        lifecycle:
          # waiting during preStop ensures no new request will hit the Terminating pod
          # on clusters using kube-proxy (kube-proxy syncs the node iptables rules every 30s)
          preStop:
            exec:
              command:
                - teleport
                - wait
                - duration
                - 30s
{{- if $proxy.postStart.command }}
          postStart:
            exec:
              command: {{ toYaml $proxy.postStart.command | nindent 14 }}
{{- end }}
{{- if $proxy.resources }}
        resources:
  {{- toYaml $proxy.resources | nindent 10 }}
{{- end }}
{{- if $proxy.securityContext }}
        securityContext: {{- toYaml $proxy.securityContext | nindent 10 }}
{{- end }}
        volumeMounts:
{{- if or $proxy.highAvailability.certManager.enabled $proxy.tls.existingSecretName }}
        - mountPath: /etc/teleport-tls
          name: "teleport-tls"
          readOnly: true
{{- end }}
{{- if $proxy.tls.existingCASecretName }}
        - mountPath: /etc/teleport-tls-ca
          name: "teleport-tls-ca"
          readOnly: true
{{- end }}
        - mountPath: /etc/teleport
          name: "config"
          readOnly: true
        - mountPath: /var/lib/teleport
          name: "data"
{{- if $projectedServiceAccountToken }}
        - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
          name: proxy-serviceaccount-token
          readOnly: true
{{- end }}
{{- if $proxy.extraVolumeMounts }}
  {{- toYaml $proxy.extraVolumeMounts | nindent 8 }}
{{- end }}
{{- if $proxy.extraContainers }}
  {{- toYaml $proxy.extraContainers | nindent 6 }}
{{- end }}
{{- if $projectedServiceAccountToken }}
      automountServiceAccountToken: false
{{- end }}
      volumes:
{{- if $projectedServiceAccountToken }}
      # This projected token volume mimics the `automountServiceAccountToken`
      # behaviour but defaults to a 1h TTL instead of 1y.
      - name: proxy-serviceaccount-token
        projected:
          sources:
            - serviceAccountToken:
                path: token
            - configMap:
                items:
                - key: ca.crt
                  path: ca.crt
                name: kube-root-ca.crt
            - downwardAPI:
                items:
                  - path: "namespace"
                    fieldRef:
                      fieldPath: metadata.namespace
{{- end }}
{{- if $proxy.highAvailability.certManager.enabled }}
      - name: teleport-tls
        secret:
          secretName: teleport-tls
{{- else if $proxy.tls.existingSecretName }}
      - name: teleport-tls
        secret:
          secretName: {{ $proxy.tls.existingSecretName }}
{{- end }}
{{- if $proxy.tls.existingCASecretName }}
      - name: teleport-tls-ca
        secret:
          secretName: {{ $proxy.tls.existingCASecretName }}
{{- end }}
      - name: "config"
        configMap:
          name: {{ .Release.Name }}-proxy
      - name: "data"
        emptyDir: {}
{{- if $proxy.extraVolumes }}
  {{- toYaml $proxy.extraVolumes | nindent 6 }}
{{- end }}
{{- if $proxy.priorityClassName }}
      priorityClassName: {{ $proxy.priorityClassName }}
{{- end }}
{{- if $proxy.podSecurityContext }}
      securityContext: {{- toYaml $proxy.podSecurityContext | nindent 8 }}
{{- end }}
      serviceAccountName: {{ include "teleport-proxy-lib.internal.serviceAccountName" . }}
      terminationGracePeriodSeconds: {{ $proxy.terminationGracePeriodSeconds }}
{{- end }}{{/* deployment */}}
