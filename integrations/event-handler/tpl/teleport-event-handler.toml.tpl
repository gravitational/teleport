storage = "./storage"
timeout = "10s"
batch = 20

[forward.fluentd]
ca = "{{index .CaCertPath}}"
cert = "{{index .ClientCertPath}}"
key = "{{index .ClientKeyPath}}"
url = "https://localhost:8888/test.log"
session-url = "https://localhost:8888/session"

[teleport]
addr = "{{.Addr}}"
identity = "identity"
refresh.enabled = true
