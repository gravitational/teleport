/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

const teleportAuthYAML = `
teleport:
  data_dir: {{TELEPORT_DATA_DIR}}
  auth_token: {{TELEPORT_AUTH_TOKEN}}
  cache:
    enabled: {{TELEPORT_CACHE_ENABLED}}
  log:
    output: stdout

auth_service:
  license_file: {{TELEPORT_LICENSE_FILE}}
  cluster_name: local-site
  enabled: true
  listen_addr: 127.0.0.1:0
  public_addr: localhost
  tokens:
  - node,auth,proxy,app:{{TELEPORT_AUTH_TOKEN}}
  authentication:
    type: local

proxy_service:
  enabled: false

ssh_service:
  enabled: false
`

const teleportProxyYAML = `
teleport:
  data_dir: {{TELEPORT_DATA_DIR}}
  auth_servers: ['{{TELEPORT_AUTH_SERVER}}']
  auth_token: '{{TELEPORT_AUTH_TOKEN}}'
  ca_pin: '{{TELEPORT_AUTH_CA_PIN}}'
  cache:
    enabled: false
  log:
    output: stdout

auth_service:
  enabled: false

proxy_service:
  enabled: true
  tunnel_public_addr: localhost:{{PROXY_TUN_LISTEN_PORT}}
  listen_addr: 127.0.0.1:0
  web_listen_addr: {{PROXY_WEB_LISTEN_ADDR}}
  tunnel_listen_addr: {{PROXY_TUN_LISTEN_ADDR}}

ssh_service:
  enabled: false
`

const teleportSSHYAML = `
teleport:
  data_dir: {{TELEPORT_DATA_DIR}}
  auth_servers: ['{{TELEPORT_AUTH_SERVER}}']
  auth_token: '{{TELEPORT_AUTH_TOKEN}}'
  ca_pin: '{{TELEPORT_AUTH_CA_PIN}}'
  cache:
    enabled: false
  log:
    output: stdout

auth_service:
  enabled: false

proxy_service:
  enabled: false

ssh_service:
  enabled: true
  listen_addr: {{SSH_LISTEN_ADDR}}
  public_addr: localhost:{{SSH_LISTEN_PORT}}
`
