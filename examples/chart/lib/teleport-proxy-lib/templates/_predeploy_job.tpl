{{- define "teleport-proxy-lib.internal.predeploy_job" }}
{{- if .Values.validateConfigOnDeploy }}
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Release.Name }}-proxy-test
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.job }}
    {{- toYaml .Values.extraLabels.job | nindent 4 }}
    {{- end }}
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "5"
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
spec:
  backoffLimit: 1
  template:
    metadata:
      labels:
        {{- include "teleport-proxy-lib.internal.labels" . | nindent 8 }}
        {{- if .Values.extraLabels.jobPod }}
        {{- toYaml .Values.extraLabels.jobPod | nindent 8 }}
        {{- end }}
    spec:
{{- if .Values.affinity }}
      affinity: {{- toYaml .Values.affinity | nindent 8 }}
{{- end }}
{{- if .Values.tolerations }}
      tolerations: {{- toYaml .Values.tolerations | nindent 6 }}
{{- end }}
{{- if .Values.imagePullSecrets }}
      imagePullSecrets:
  {{- toYaml .Values.imagePullSecrets | nindent 6 }}
{{- end }}
      restartPolicy: Never
      containers:
      - name: "teleport"
        image: '{{ if .Values.enterprise }}{{ .Values.enterpriseImage }}{{ else }}{{ .Values.image }}{{ end }}:{{ include "teleport-proxy-lib.internal.version" . }}'
        imagePullPolicy: {{ .Values.imagePullPolicy }}
{{- if .Values.jobResources }}
        resources:
  {{- toYaml .Values.jobResources | nindent 10 }}
{{- end }}
{{- if or .Values.extraEnv .Values.tls.existingCASecretName }}
        env:
  {{- if (gt (len .Values.extraEnv) 0) }}
    {{- toYaml .Values.extraEnv | nindent 8 }}
  {{- end }}
  {{- if .Values.tls.existingCASecretName }}
        - name: SSL_CERT_FILE
          value: "/etc/teleport-tls-ca/{{ required "tls.existingCASecretKeyName must be set if tls.existingCASecretName is set in chart values" .Values.tls.existingCASecretKeyName }}"
  {{- end }}
{{- end }}
        command:
          - "teleport"
          - "configure"
        args:
          - "--test"
          - "/etc/teleport/teleport.yaml"
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
{{- if .Values.extraVolumeMounts }}
  {{- toYaml .Values.extraVolumeMounts | nindent 8 }}
{{- end }}
      volumes:
{{- if .Values.highAvailability.certManager.enabled }}
      - name: teleport-tls
        secret:
          secretName: teleport-tls
          # this avoids deadlock during initial setup
          optional: true
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
          name: {{ .Release.Name }}-proxy-test
      - name: "data"
        emptyDir: {}
{{- if .Values.extraVolumes }}
  {{- toYaml .Values.extraVolumes | nindent 6 }}
{{- end }}
      serviceAccountName: {{ include "teleport-proxy-lib.internal.hookServiceAccountName" . }}
{{- end }}
{{- end }}{{/* predeploy_job */}}
