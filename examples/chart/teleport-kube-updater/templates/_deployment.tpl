{{- define "teleport-kube-updater.deployment" -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .releaseName }}-updater
  namespace: {{ .releaseNamespace }}
  labels:
    app: {{ .releaseName }}-updater
{{- if .extraLabels.deployment }}
    {{- toYaml .extraLabels.deployment | nindent 4 }}
{{- end }}
{{- if .annotations.deployment }}
  annotations: {{- toYaml .annotations.deployment | nindent 4 }}
{{- end }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .releaseName }}-updater
  template:
    metadata:
      annotations:
{{- if .annotations.pod }}
  {{- toYaml .annotations.pod | nindent 8 }}
{{- end }}
      labels:
        app: {{ .releaseName }}-updater
{{- if .extraLabels.pod }}
  {{- toYaml .extraLabels.pod | nindent 8 }}
{{- end }}
    spec:
{{- if .affinity }}
      affinity: {{- toYaml .affinity | nindent 8 }}
{{- end }}
{{- if .tolerations }}
      tolerations: {{- toYaml .tolerations | nindent 8 }}
{{- end }}
{{- if .imagePullSecrets }}
      imagePullSecrets: {{- toYaml .imagePullSecrets | nindent 8 }}
{{- end }}
{{- if .nodeSelector }}
      nodeSelector: {{- toYaml .nodeSelector | nindent 8 }}
{{- end }}
      containers:
      - name: "kube-agent-updater"
        image: "{{ .image }}:{{ .version }}"
{{- if .imagePullPolicy }}
        imagePullPolicy: {{ toYaml .imagePullPolicy }}
{{- end }}
{{- if or .extraEnv .tls.existingCASecretName }}
        env:
  {{- if (gt (len .extraEnv) 0) }}
    {{- toYaml .extraEnv | nindent 8 }}
  {{- end }}
  {{- if .tls.existingCASecretName }}
        - name: SSL_CERT_FILE
          value: "/etc/teleport-tls-ca/{{ required "tls.existingCASecretKeyName must be set if tls.existingCASecretName is set in chart values" .tls.existingCASecretKeyName }}"
  {{- end }}
{{- end }}
        args:
          - "--agent-name={{ .releaseName }}"
          - "--agent-namespace={{ .releaseNamespace }}"
          - "--base-image={{ .baseImage }}"
  {{- if .versionServer}}
          - "--version-server={{ .versionServer }}"
          - "--version-channel={{ .releaseChannel }}"
  {{- end }}
  {{- /* We don't want to enable the RFD-184 update protocol if the user has set a custom versionServer as this
    would be a breaking change when the teleport proxy starts override the explicitly set RFD-109 version server */ -}}
  {{- if and .proxyAddr (not .versionServerOverride)}}
          - "--proxy-address={{ .proxyAddr }}"
          - "--update-group={{ default "default" .group }}"
  {{- end }}
{{- if .pullCredentials }}
          - "--pull-credentials={{ .pullCredentials }}"
{{- end }}
{{- if .extraArgs }}
          {{- toYaml .extraArgs | nindent 10 }}
{{- end }}
{{- if .securityContext }}
        securityContext: {{- toYaml .securityContext | nindent 10 }}
{{- end }}
        ports:
        - name: metrics
          containerPort: 8080
          protocol: TCP
        - name: healthz
          containerPort: 8081
          protocol: TCP
        livenessProbe:
          httpGet:
            path: /healthz
            port: healthz
          initialDelaySeconds: 5
          periodSeconds: 5
          failureThreshold: 6 # consider agent unhealthy after 30s (6 * 5s)
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /readyz
            port: healthz
          initialDelaySeconds: 5
          periodSeconds: 5
          failureThreshold: 6 # consider unready after 30s
          timeoutSeconds: 5
{{- if .resources }}
        resources: {{- toYaml .resources | nindent 10 }}
{{- end }}
{{- if or .tls.existingCASecretName .extraVolumeMounts }}
        volumeMounts:
  {{- if .tls.existingCASecretName }}
          - mountPath: /etc/teleport-tls-ca
            name: "teleport-tls-ca"
            readOnly: true
  {{- end }}
  {{- if .extraVolumeMounts }}
          {{- toYaml .extraVolumeMounts | nindent 10 }}
  {{- end }}
{{- end }}
{{- if or .tls.existingCASecretName .extraVolumes }}
      volumes:
  {{- if .extraVolumes }}
        {{- toYaml .extraVolumes | nindent 8 }}
  {{- end }}
  {{- if .tls.existingCASecretName }}
        - name: "teleport-tls-ca"
          secret:
            secretName: {{ .tls.existingCASecretName }}
  {{- end }}
{{- end }}
{{- if .priorityClassName }}
      priorityClassName: {{ .priorityClassName }}
{{- end }}
      serviceAccountName: {{ .serviceAccount.name }}
{{- end -}}