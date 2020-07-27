/*
Copyright 2015 Gravitational, Inc.

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

package config

const StaticConfigString = `
#
# Some comments
#
teleport:
  nodename: edsger.example.com
  advertise_ip: 10.10.10.1:3022
  pid_file: /var/run/teleport.pid
  auth_servers:
    - auth0.server.example.org:3024
    - auth1.server.example.org:3024
  auth_token: xxxyyy
  log:
    output: stderr
    severity: INFO
  storage:
    type: etcd
    peers: ['one', 'two']
    tls_key_file: /tls.key
    tls_cert_file: /tls.cert
    tls_ca_file: /tls.ca
  connection_limits:
    max_connections: 90
    max_users: 91
    rates:
    - period: 1m1s
      average: 70
      burst: 71
    - period: 10m10s
      average: 170
      burst: 171
  cache:
    enabled: yes
    ttl: 20h

auth_service:
  enabled: yes
  listen_addr: auth:3025
  tokens:
  - "proxy,node:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  - "auth:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  authorities:
  - type: host
    domain_name: example.com
    checking_keys:
      - checking key 1
    checking_key_files:
      - /ca.checking.key
    signing_keys:
      - !!binary c2lnbmluZyBrZXkgMQ==
    signing_key_files:
      - /ca.signing.key
  reverse_tunnels:
      - domain_name: tunnel.example.com
        addresses: ["com-1", "com-2"]
      - domain_name: tunnel.example.org
        addresses: ["org-1"]
  public_addr: ["auth.default.svc.cluster.local:3080"]
  disconnect_expired_cert: yes
  client_idle_timeout: 17s

ssh_service:
  enabled: no
  listen_addr: ssh:3025
  labels:
    name: mongoserver
    role: follower
  commands:
  - name: hostname
    command: [/bin/hostname]
    period: 10ms
  - name: date
    command: [/bin/date]
    period: 20ms
  public_addr: "luna3:22"
`

const SmallConfigString = `
teleport:
  nodename: cat.example.com
  advertise_ip: 10.10.10.1
  pid_file: /var/run/teleport.pid
  auth_token: %v
  auth_servers:
    - auth0.server.example.org:3024
    - auth1.server.example.org:3024
  log:
    output: stderr
    severity: INFO
  connection_limits:
    max_connections: 90
    max_users: 91
    rates:
    - period: 1m1s
      average: 70
      burst: 71
    - period: 10m10s
      average: 170
      burst: 171
auth_service:
  enabled: yes
  listen_addr: 10.5.5.1:3025
  cluster_name: magadan
  tokens:
  - "proxy,node:xxx"
  - "auth:yyy"
ssh_service:
  enabled: no

proxy_service:
  enabled: yes
  web_listen_addr: webhost
  tunnel_listen_addr: tunnelhost:1001
  public_addr: web3:443
`

// NoServicesConfigString is a configuration file with no services enabled
// but with values for all services set.
const NoServicesConfigString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: no
  cluster_name: "example.com"
  public_addr: "auth.example.com"

ssh_service:
  enabled: no
  public_addr: "ssh.example.com"

proxy_service:
  enabled: no
  public_addr: "proxy.example.com"
`

// LegacyAuthenticationSection is the deprecated format for authentication method. We still
// need to support it until it's fully removed.
const LegacyAuthenticationSection = `
auth_service:
  oidc_connectors:
    - id: google
      redirect_url: https://localhost:3080/v1/webapi/oidc/callback
      client_id: id-from-google.apps.googleusercontent.com
      client_secret: secret-key-from-google
      issuer_url: https://accounts.google.com
  u2f:
    enabled: "yes"
    app_id: https://graviton:3080
    facets:
    - https://graviton:3080
`

// configWithFIPSKex is a configuration file with a FIPS compliant KEX
// algorithm.
const configWithFIPSKex = `
teleport:
  kex_algos:
    - ecdh-sha2-nistp256
auth_service:
  enabled: yes
  authentication:
    type: saml
    local_auth: false
`

// configWithoutFIPSKex is a configuration file without a FIPS compliant KEX
// algorithm.
const configWithoutFIPSKex = `
teleport:
  kex_algos:
    - curve25519-sha256@libssh.org
auth_service:
  enabled: yes
  authentication:
    type: saml
    local_auth: false
`
