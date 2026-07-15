storage = "./storage"
timeout = "10s"
batch = 20

[forward.fluentd]
ca = "{{index .CaCertPath}}"
cert = "{{index .ClientCertPath}}"
key = "{{index .ClientKeyPath}}"
url = "https://localhost:8888/test.log"
# Note: .<session-id>.log is appended to session-url for each session recording.
# Ensure your log collector's tag matching accounts for this (e.g., session.* in Fluentd/Fluent Bit).
session-url = "https://localhost:8888/session"

[teleport]
addr = "{{.Addr}}"
identity = "identity"
refresh.enabled = true
