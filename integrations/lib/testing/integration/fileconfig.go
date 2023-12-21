/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
