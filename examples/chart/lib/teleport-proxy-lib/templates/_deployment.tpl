{{- define "teleport-proxy-lib.internal.deployment" }}
{{- $replicable := or .Values.highAvailability.certManager.enabled .Values.tls.existingSecretName .Values.ingress.enabled -}}
{{- $projectedServiceAccountToken := semverCompare ">=1.20.0-0" .Capabilities.KubeVersion.Version }}
{{- $topologySpreadConstraints := and (semverCompare ">=1.18.0-0" .Capabilities.KubeVersion.Version) (not .Values.disableTopologySpreadConstraints) }}
# Deployment is {{ if not $replicable }}not {{end}}replicable
{{- if and .Values.highAvailability.certManager.enabled .Values.tls.existingSecretName }}
{{- fail "Cannot set both highAvailability.certManager.enabled and tls.existingSecretName, choose one or the other" }}
{{- end }}
{{- if and .Values.acme .Values.tls.existingSecretName }}
{{- fail "Cannot set both acme.enabled and tls.existingSecretName, choose one or the other" }}
{{- end }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-proxy
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.deployment }}
    {{- toYaml .Values.extraLabels.deployment | nindent 4 }}
    {{- end }}
{{- if .Values.annotations.deployment }}
  annotations: {{- toYaml .Values.annotations.deployment | nindent 4 }}
{{- end }}
spec:
{{- /*
  If proxies cannot be replicated we use a single replica.
  By default we want to upgrade all users to at least 2 replicas, if they had a higher replica count we take it.
  Users want to force a single proxy can use forceHAReplicas.
*/}}
{{- if $replicable }}
  {{- if .Values.forceHAReplicas }}
  replicas: {{ .Values.highAvailability.replicaCount }}
  {{- else }}
  replicas: {{ max .Values.highAvailability.replicaCount 2 }}
  {{- end }}
  {{- if .Values.highAvailability.minReadySeconds }}
  minReadySeconds: {{ .Values.highAvailability.minReadySeconds }}
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
{{- if .Values.annotations.pod }}
  {{- toYaml .Values.annotations.pod | nindent 8 }}
{{- end }}
      labels:
        {{- include "teleport-proxy-lib.internal.labels" . | nindent 8 }}
        {{- if .Values.extraLabels.pod }}
        {{- toYaml .Values.extraLabels.pod | nindent 8 }}
        {{- end }}
    spec:
{{- if .Values.nodeSelector }}
      nodeSelector: {{- toYaml .Values.nodeSelector | nindent 8 }}
{{- end }}
{{- if $topologySpreadConstraints }}
  {{- if .Values.topologySpreadConstraints }}
      topologySpreadConstraints: {{- toYaml .Values.topologySpreadConstraints | nindent 8 }}
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
{{- if .Values.affinity }}
  {{- if .Values.highAvailability.requireAntiAffinity }}
    {{- fail "Cannot use highAvailability.requireAntiAffinity when affinity is also set in chart values - unset one or the other" }}
  {{- end }}
  {{- toYaml .Values.affinity | nindent 8 }}
{{- else }}
        podAntiAffinity:
  {{- if .Values.highAvailability.requireAntiAffinity }}
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
  {{- else if gt (int .Values.highAvailability.replicaCount) 1 }}
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
{{- if .Values.tolerations }}
      tolerations: {{- toYaml .Values.tolerations | nindent 6 }}
{{- end }}
{{- if .Values.imagePullSecrets }}
      imagePullSecrets:
  {{- toYaml .Values.imagePullSecrets | nindent 6 }}
{{- end }}
{{- if .Values.initContainers }}
      initContainers:
  {{- $proxyContext := . -}}{{/* For use inside the range loop */}}
  {{- range $initContainer := .Values.initContainers }}
    {{- if and (not $initContainer.resources) $proxyContext.Values.resources }}
      {{- $_ := set $initContainer "resources" $proxyContext.Values.resources }}
    {{- end }}
    {{- $skipDefaultVolumeMounts := $initContainer.skipDefaultVolumeMounts | default false }}
    {{- list (omit $initContainer "skipDefaultVolumeMounts") | toYaml | nindent 8 }}
    {{- if not $skipDefaultVolumeMounts }}
          volumeMounts:
    {{- if or $proxyContext.Values.highAvailability.certManager.enabled $proxyContext.Values.tls.existingSecretName }}
          - mountPath: /etc/teleport-tls
            name: "teleport-tls"
            readOnly: true
    {{- end }}{{/* HA or existingSecretName */}}
          - mountPath: /etc/teleport
            name: "config"
            readOnly: true
          - mountPath: /var/lib/teleport
            name: "data"
    {{- if $proxyContext.Values.extraVolumeMounts }}
      {{- toYaml $proxyContext.Values.extraVolumeMounts | nindent 10 }}
    {{- end }}{{/* extraVolumeMounts */}}
    {{- end }}
  {{- end }}{{/* range .Values.initContainers */}}
{{- end }}{{/* initContainers */}}
      containers:
      - name: "teleport"
        image: '{{ if .Values.enterprise }}{{ .Values.enterpriseImage }}{{ else }}{{ .Values.image }}{{ end }}:{{ include "teleport-proxy-lib.internal.version" . }}'
        imagePullPolicy: {{ .Values.imagePullPolicy }}
        {{- $gomemlimit := include "teleport-util-lib.gomemlimit" .Values }}
        {{- if or .Values.extraEnv .Values.tls.existingCASecretName $gomemlimit }}
        env:
        {{- if $gomemlimit }}
        - name: GOMEMLIMIT
          value: {{ $gomemlimit | quote }}
        {{- end }}
        {{- if (gt (len .Values.extraEnv) 0) }}
          {{- toYaml .Values.extraEnv | nindent 8 }}
        {{- end }}
        {{- if .Values.tls.existingCASecretName }}
        - name: SSL_CERT_FILE
          value: "/etc/teleport-tls-ca/{{ required "tls.existingCASecretKeyName must be set if tls.existingCASecretName is set in chart values" .Values.tls.existingCASecretKeyName }}"
        {{- end }}
        {{- end }}
        args:
        - "--diag-addr=0.0.0.0:3000"
        {{- if .Values.insecureSkipProxyTLSVerify }}
        - "--insecure"
        {{- end }}
        {{- if .Values.extraArgs }}
          {{- toYaml .Values.extraArgs | nindent 8 }}
        {{- end }}
        ports:
        - name: tls
          containerPort: 3080
          protocol: TCP
        {{- if .Values.enterprise }}
        - name: proxypeering
          containerPort: 3021
          protocol: TCP
        {{- end }}
        {{- if ne .Values.proxyListenerMode "multiplex" }}
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
        {{- if .Values.separatePostgresListener }}
        - name: postgres
          containerPort: 5432
          protocol: TCP
        {{- end }}
        {{- if .Values.separateMongoListener }}
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
          timeoutSeconds: {{ .Values.probeTimeoutSeconds }}
        readinessProbe:
          httpGet:
            path: /readyz
            port: diag
          initialDelaySeconds: {{ .Values.readinessProbe.initialDelaySeconds }}
          periodSeconds: {{ .Values.readinessProbe.periodSeconds }}
          failureThreshold: {{.Values.readinessProbe.failureThreshold}}
          successThreshold: {{.Values.readinessProbe.successThreshold}}
          timeoutSeconds: {{ .Values.probeTimeoutSeconds }}
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
{{- if .Values.postStart.command }}
          postStart:
            exec:
              command: {{ toYaml .Values.postStart.command | nindent 14 }}
{{- end }}
{{- if .Values.resources }}
        resources:
  {{- toYaml .Values.resources | nindent 10 }}
{{- end }}
{{- if .Values.securityContext }}
        securityContext: {{- toYaml .Values.securityContext | nindent 10 }}
{{- end }}
        volumeMounts:
{{- if or .Values.highAvailability.certManager.enabled .Values.tls.existingSecretName }}
        - mountPath: /etc/teleport-tls
          name: "teleport-tls"
          readOnly: true
{{- end }}
{{- if .Values.tls.existingCASecretName }}
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
{{- if .Values.extraVolumeMounts }}
  {{- toYaml .Values.extraVolumeMounts | nindent 8 }}
{{- end }}
        {{- if .Values.extraContainers }}
  {{- toYaml .Values.extraContainers | nindent 6 }}
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
{{- if .Values.highAvailability.certManager.enabled }}
      - name: teleport-tls
        secret:
          secretName: teleport-tls
{{- else if .Values.tls.existingSecretName }}
      - name: teleport-tls
        secret:
          secretName: {{ .Values.tls.existingSecretName }}
{{- end }}
{{- if .Values.tls.existingCASecretName }}
      - name: teleport-tls-ca
        secret:
          secretName: {{ .Values.tls.existingCASecretName }}
{{- end }}
      - name: "config"
        configMap:
          name: {{ .Release.Name }}-proxy
      - name: "data"
        emptyDir: {}
{{- if .Values.extraVolumes }}
  {{- toYaml .Values.extraVolumes | nindent 6 }}
{{- end }}
{{- if .Values.priorityClassName }}
      priorityClassName: {{ .Values.priorityClassName }}
{{- end }}
{{- if .Values.podSecurityContext }}
      securityContext: {{- toYaml .Values.podSecurityContext | nindent 8 }}
{{- end }}
      serviceAccountName: {{ include "teleport-proxy-lib.internal.serviceAccountName" . }}
      terminationGracePeriodSeconds: {{ .Values.terminationGracePeriodSeconds }}
{{- end }}{{/* deployment */}}
