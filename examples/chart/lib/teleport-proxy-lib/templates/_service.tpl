{{- define "teleport-proxy-lib.internal.service" }}
{{- $backendProtocol := ternary "ssl" "tcp" (hasKey .Values.annotations.service "service.beta.kubernetes.io/aws-load-balancer-ssl-cert") -}}
{{- /* Fail early if proxy service type is set to LoadBalancer when ingress.enabled=true */ -}}
{{- if and .Values.ingress.enabled (eq .Values.service.type "LoadBalancer") -}}
  {{- fail "proxy.service.type must not be LoadBalancer when using an ingress - any load balancer should be provisioned by your ingress controller. Set proxy.service.type=ClusterIP instead" -}}
{{- end -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "teleport-proxy-lib.internal.labels" . | nindent 4 }}
    {{- if .Values.extraLabels.service }}
    {{- toYaml .Values.extraLabels.service | nindent 4 }}
    {{- end }}
  {{- if (or (.Values.annotations.service) (eq .Values.chartMode "aws")) }}
  annotations:
    {{- if and (eq .Values.chartMode "aws") (not .Values.ingress.enabled) }}
      {{- if not (hasKey .Values.annotations.service "service.beta.kubernetes.io/aws-load-balancer-backend-protocol")}}
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: {{ $backendProtocol }}
      {{- end }}
      {{- if not (or (hasKey .Values.annotations.service "service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled") (hasKey .Values.annotations.service "service.beta.kubernetes.io/aws-load-balancer-attributes"))}}
    service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
      {{- end }}
      {{- if not (hasKey .Values.annotations.service "service.beta.kubernetes.io/aws-load-balancer-type")}}
    service.beta.kubernetes.io/aws-load-balancer-type: nlb
      {{- end }}
    {{- end }}
    {{- if .Values.annotations.service }}
      {{- toYaml .Values.annotations.service | nindent 4 }}
    {{- end }}
  {{- end }}
spec:
  type: {{ default "LoadBalancer" .Values.service.type }}
{{- with .Values.service.spec }}
  {{- toYaml . | nindent 2 }}
{{- end }}
  ports:
  - name: tls
    port: 443
    targetPort: 3080
    protocol: TCP
{{- if ne .Values.proxyListenerMode "multiplex" }}
  - name: sshproxy
    port: 3023
    targetPort: 3023
    protocol: TCP
  - name: k8s
    port: 3026
    targetPort: 3026
    protocol: TCP
  - name: sshtun
    port: 3024
    targetPort: 3024
    protocol: TCP
  - name: mysql
    port: 3036
    targetPort: 3036
    protocol: TCP
  {{- if .Values.separatePostgresListener }}
  - name: postgres
    port: 5432
    targetPort: 5432
    protocol: TCP
  {{- end }}
  {{- if .Values.separateMongoListener }}
  - name: mongo
    port: 27017
    targetPort: 27017
    protocol: TCP
  {{- end }}
{{- end }}
  selector: {{- include "teleport-proxy-lib.internal.selectorLabels" . | nindent 4 }}
{{- end }}{{/* service */}}
