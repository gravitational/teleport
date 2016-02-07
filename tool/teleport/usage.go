package main

const (
	usageNotes = `
Notes:

  --no-ssh=false

  When set, Teleport does not start the SSH service, only allowing proxied connections to 
  other SSH nodes.

  --proxy-addr=<host>[:port]/<token>

  Tells teleport to run as an SSH node behind the given proxy host. You need to obtain the 
  token by executing "tctl nodes add" on the host where Teleport proxy is running.

  --advertise-ip=ip_address

  When connecting to a proxy from behind a NAT, this tells the proxy which IP to find 
  this node on. 
`

	usageExamples = `
Examples:

> teleport start

  Without cofiguration, teleport starts by default in a "showcase mode": it becomes an 
  SSH server and a proxy to itself with a Web UI.

> teleport start --no-ssh

  Starts teleport in a proxy+auth mode, serving the Web UI for 2-factor auth. You must
  execute 'tctl nodes add' now to generate one-time tokens to add nodes to the cluster.

> teleport start --proxy-addr=bastion.host:3023/token \
                 --listen_interface=0.0.0.0 \
                 --advertise_interface=10.0.1.50

  Starts teleport as an SSH node connected to the SSH proxy/bastion on bastion.host:3023
  Tells the proxy that this node is reachable via 10.0.1.50
`

	sampleConfig = `##
## This is the example of a Teleport configuration file with all settings
## set to their default value. Uncomment & customize as needed.
##
#global:
#    hostname:localhost
#    listen_interface:0.0.0.0
#    advertize_interface:auto
#    proxy_addr: 127.0.0.1:3023
#    proxy_token: ""
#    auth_servers: ["tcp://127.0.0.1:3024"]
#    connection_limits:
#    max: 100
#    rates:
#      - period: 10s
#        average: 5
#        burst: 10
#    storage:
#        type: bolt
#        params: { path: "/var/lib/teleport" }
#    log:
#        output: stderr  
#        severity: INFO  
#
#auth_service:
#   enabled: yes
#   listen_addr: tcp://127.0.0.1:3024
#
#
#ssh_service:
#   enabled: yes
#   token: “”
#   proxy_addr: tcp://127.0.0.1:3023
#   labels:
#       name:value
#       name2:value2
#   label-commands:
#       os:
#           period: 1m
#           command: ["uname", "-r"]
#
#proxy_service:
#   enabled: yes
#   https_only: true
#   insecure_http_addr: tcp://0.0.0.0:3080
#   https_addr: tcp://0.0.0.0:3081
#   https_key_file: ""
#   https_crt_file: ""
#   ssh_addr: tcp://0.0.0.0:3023
#   auth_token: ""
`
)
