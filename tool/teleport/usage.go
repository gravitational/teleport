package main

const (
	usageNotes = `Notes:
  --roles=node,proxy,auth

  This flag tells Teleport which services to run. By default it runs all three. 
  In a production environment you may want to separate them.

  --token=xyz

  This token is needed to connect a node to an auth server. Obtain it by running 
  "tctl nodes add" on the auth server. It's used once and ignored afterwards.
`

	usageExamples = `
Examples:

> teleport start
      By default without any configuration, teleport starts running with all roles 
      enabled It's the equivalent of running with --roles=node,proxy,auth 

> teleport start --roles=node --auth-server=10.5.0.2 --token=xyz --name=database
      Starts this SSH node named 'database' running in SSH server mode and 
      authenticating connections via the auth server running on 10.5.0.2`

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
