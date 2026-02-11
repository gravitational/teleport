eventHandler:
  storagePath: "./storage"
  timeout: "10s"
  batch: 20

teleport:
  address: "{{.Addr}}"
  identitySecretName: teleport-event-handler-identity
  identitySecretPath: identity

fluentd:
  url: "https://fluentd.fluentd.svc.cluster.local/events.log"
  sessionUrl: "https://fluentd.fluentd.svc.cluster.local/session.log"
  certificate:
    secretName: "teleport-event-handler-client-tls"
    caPath: "ca.crt"
    certPath: "client.crt"
    keyPath: "client.key"

persistentVolumeClaim:
  enabled: true
