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

package config

const StaticConfigString = `
#
# Some comments
#
version: v3
teleport:
  nodename: edsger.example.com
  advertise_ip: 10.10.10.1:3022
  pid_file: /var/run/teleport.pid
  auth_server: auth0.server.example.org:3024
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
    max_backoff: 12m

auth_service:
  enabled: yes
  listen_addr: auth:3025
  tokens:
  - "proxy,node:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
  - "auth:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  reverse_tunnels:
      - domain_name: tunnel.example.com
        addresses: ["com-1", "com-2"]
      - domain_name: tunnel.example.org
        addresses: ["org-1"]
  public_addr: ["auth.default.svc.cluster.local:3080"]
  disconnect_expired_cert: yes
  client_idle_timeout: 17s
  routing_strategy: most_recent

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
version: v3
teleport:
  nodename: cat.example.com
  advertise_ip: 10.10.10.1
  pid_file: /var/run/teleport.pid
  auth_token: %v
  auth_server: auth0.server.example.org:3024
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
  diag_addr: 127.0.0.1:3000
  ca_pin:
    - ca-pin-from-string
    - %v
auth_service:
  enabled: yes
  listen_addr: 10.5.5.1:3025
  cluster_name: magadan
  tokens:
  - "proxy,node:xxx"
  - "node:%v"
  - "auth:yyy"
  ca_key_params:
    pkcs11:
      module_path: %s
      token_label: "example_token"
      slot_number: 1
      pin: "example_pin"
  authentication:
    second_factor: "webauthn"
    webauthn:
      rp_id: "goteleport.com"
      attestation_allowed_cas:
      - "testdata/u2f_attestation_ca.pem"
      - |
        -----BEGIN CERTIFICATE-----
        MIIDFzCCAf+gAwIBAgIDBAZHMA0GCSqGSIb3DQEBCwUAMCsxKTAnBgNVBAMMIFl1
        YmljbyBQSVYgUm9vdCBDQSBTZXJpYWwgMjYzNzUxMCAXDTE2MDMxNDAwMDAwMFoY
        DzIwNTIwNDE3MDAwMDAwWjArMSkwJwYDVQQDDCBZdWJpY28gUElWIFJvb3QgQ0Eg
        U2VyaWFsIDI2Mzc1MTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMN2
        cMTNR6YCdcTFRxuPy31PabRn5m6pJ+nSE0HRWpoaM8fc8wHC+Tmb98jmNvhWNE2E
        ilU85uYKfEFP9d6Q2GmytqBnxZsAa3KqZiCCx2LwQ4iYEOb1llgotVr/whEpdVOq
        joU0P5e1j1y7OfwOvky/+AXIN/9Xp0VFlYRk2tQ9GcdYKDmqU+db9iKwpAzid4oH
        BVLIhmD3pvkWaRA2H3DA9t7H/HNq5v3OiO1jyLZeKqZoMbPObrxqDg+9fOdShzgf
        wCqgT3XVmTeiwvBSTctyi9mHQfYd2DwkaqxRnLbNVyK9zl+DzjSGp9IhVPiVtGet
        X02dxhQnGS7K6BO0Qe8CAwEAAaNCMEAwHQYDVR0OBBYEFMpfyvLEojGc6SJf8ez0
        1d8Cv4O/MA8GA1UdEwQIMAYBAf8CAQEwDgYDVR0PAQH/BAQDAgEGMA0GCSqGSIb3
        DQEBCwUAA4IBAQBc7Ih8Bc1fkC+FyN1fhjWioBCMr3vjneh7MLbA6kSoyWF70N3s
        XhbXvT4eRh0hvxqvMZNjPU/VlRn6gLVtoEikDLrYFXN6Hh6Wmyy1GTnspnOvMvz2
        lLKuym9KYdYLDgnj3BeAvzIhVzzYSeU77/Cupofj093OuAswW0jYvXsGTyix6B3d
        bW5yWvyS9zNXaqGaUmP3U9/b6DlHdDogMLu3VLpBB9bm5bjaKWWJYgWltCVgUbFq
        Fqyi4+JE014cSgR57Jcu3dZiehB6UtAPgad9L5cNvua/IWRmm+ANy3O2LH++Pyl8
        SREzU8onbBsjMg9QDiSf5oJLKvd/Ren+zGY7
        -----END CERTIFICATE-----

ssh_service:
  enabled: no

proxy_service:
  enabled: yes
  web_listen_addr: webhost
  tunnel_listen_addr: tunnelhost:1001
  peer_listen_addr: peerhost:1234
  peer_public_addr: peer.example:1234
  public_addr: web3:443
  postgres_public_addr: postgres.example:5432
  mysql_listen_addr: webhost:3336
  mysql_public_addr: mysql.example:3306
  mongo_listen_addr: webhost:27017
  mongo_public_addr: mongo.example:27017

db_service:
  enabled: yes
  resources:
    - labels:
        "*": "*"
      aws:
        assume_role_arn: "arn:aws:iam::123456789012:role/DBAccess"
        external_id: "externalID123"
  azure:
    - subscriptions: ["sub1", "sub2"]
      resource_groups: ["group1", "group2"]
      types: ["postgres", "mysql"]
      regions: ["eastus", "centralus"]
      tags:
        "a": "b"
    - types: ["postgres", "mysql"]
      regions: ["westus"]
      tags:
        "c": "d"
  aws:
      - types: ["rds"]
        regions: ["us-west-1"]
        assume_role_arn: "arn:aws:iam::123456789012:role/DBDiscoverer"
        external_id: "externalID123"

kubernetes_service:
    enabled: yes
    resources:
      - labels:
          "*": "*"
    kubeconfig_file: /tmp/kubeconfig
    labels:
      'testKey': 'testValue'

discovery_service:
    enabled: yes
    aws:
      - types: ["ec2"]
        regions: ["eu-central-1"]
        assume_role_arn: "arn:aws:iam::123456789012:role/DBDiscoverer"
        external_id: "externalID123"

okta_service:
    enabled: yes
    api_endpoint: https://some-endpoint
    api_token_path: %v
    sync_period: 300s
    sync:
      sync_access_lists: yes
      default_owners:
      - owner1
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

app_service:
  enabled: no
`

// DefaultAuthResourcesConfigString is a configuration file without
// `cluster_auth_preference`, `cluster_networking_config` and `session_recording` fields.
const DefaultAuthResourcesConfigString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  cluster_name: "example.com"
`

// CustomAuthPreferenceConfigString is a configuration file with a single
// `cluster_auth_preference` field.
const CustomAuthPreferenceConfigString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  cluster_name: "example.com"
  disconnect_expired_cert: true
`

// AuthPreferenceConfigWithMOTDString is a configuration file with the
// `message_of_the_day` `cluster_auth_preference` field.
const AuthPreferenceConfigWithMOTDString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  cluster_name: "example.com"
  message_of_the_day: "welcome!"
`

// CustomNetworkingConfigString is a configuration file with a single
// `cluster_networking_config` field.
const CustomNetworkingConfigString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  cluster_name: "example.com"
  web_idle_timeout: 10s
`

// CustomSessionRecordingConfigString is a configuration file with a single
// `session_recording` field.
const CustomSessionRecordingConfigString = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  cluster_name: "example.com"
  proxy_checks_host_keys: true
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

const configWithCAPins = `
teleport:
  nodename: cat.example.com
  advertise_ip: 10.10.10.1
  pid_file: /var/run/teleport.pid
  log:
    output: stderr
    severity: INFO
  ca_pin: [%v]
auth_service:
  enabled: yes
  listen_addr: 10.5.5.1:3025
  cluster_name: magadan
  tokens:
  - "proxy,node:xxx"
  - "auth:yyy"
  authentication:
    type: local
    second_factors: [otp]

ssh_service:
  enabled: no

proxy_service:
  enabled: yes
  web_listen_addr: webhost
  tunnel_listen_addr: tunnelhost:1001
  public_addr: web3:443
`

const configSessionRecording = `
teleport:
  nodename: node.example.com

auth_service:
  enabled: yes
  %v
  %v

ssh_service:
  enabled: no
  public_addr: "ssh.example.com"

proxy_service:
  enabled: no
  public_addr: "proxy.example.com"

app_service:
  enabled: no
`
