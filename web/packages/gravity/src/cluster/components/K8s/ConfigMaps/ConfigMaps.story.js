/*
Copyright 2019 Gravitational, Inc.

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

import React from 'react'
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import { ConfigMaps } from './ConfigMaps'

storiesOf('Gravity/K8s', module)
  .add('ConfigMaps', () => {
    return (
      <ConfigMaps {...props} />
    );
  })
  .add('ConfigMaps Empty', () => {
    const newProps = {
      ...props,
      configMaps: []
    }

    return (
      <ConfigMaps {...newProps} />
    );
  });

const props = {
  siteId: 'xxx',
  namespace: 'kube-system',
  onFetch: () => $.Deferred(),
  configMaps:  [
    {
      "name": "extension-apiserver-authentication",
      "id": "37fa3af6-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": [
        {
          "name": "client-ca-file",
          "content": "-----BEGIN CERTIFICATE-----\nMIIDEzCCAfugAwIBAgIUMxyrsBqVBey+BxfhsKkmqpJKrJAwDQYJKoZIhvcNAQEL\nBQAwIDEeMBwGA1UEAxMVZGVtby5ncmF2aXRhdGlvbmFsLmlvMB4XDTE4MTIyMDAz\nMjEwMFoXDTM4MTIxNTAzMjEwMFowIDEeMBwGA1UEAxMVZGVtby5ncmF2aXRhdGlv\nbmFsLmlvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvL478Ld5FM37\n7nIZSQ7KVhkVI5Fg8vzmAmgwNKn60PMu4YroOFKwv02gQ0OdaN1IgekW/lmuAE+K\nRxvrNLlu3/WsImgMamZOe075h9svh3+MNyK3xBgFhy0PiO6rdmKK31xO5XvP8iVP\nOF8Rvp8NcuvazvNn8Z5eLtT0dGRaqf+4+VNPQ6oE3XXhpmgGgrC+YZwhRWdvZC/g\n+HaKFnefQvKDLrOZL37pTx9OtHVrWNvKXUcCu1my5HUMw9CoMbdKS9q+dB4AmOhe\nYVAgWnaWOl39OSiVleqotzYcT21twM/ey7k3Ziof+HNuzMewIDbp+kJj2YMxHGf2\nzY84rLqTiQIDAQABo0UwQzAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB\n/wIBAjAdBgNVHQ4EFgQUmWpjjrA32hxBzpPenOKLifd7s3YwDQYJKoZIhvcNAQEL\nBQADggEBAJ0njpjKQafud9A/Aj6+90G8uTDEQgT8gJISgTTzhixcaDeDtOvq7SR0\neDc8bWsEOF0KuNfB7QdAqhOZmURA93qQpQsnNiEvuFhIW2+G+lqHOHzJDe1uCUj/\naiZrJhSzDVVgIk6jF27UJBfZ5GZ57iRxS25evpW3lYqC27rubCpAk8jmLOt6tuFt\nfB2J9gw9SVflzKP771X98EdnMxPvXdRtQd2SYrJDzt7GOi8+MsYwCS8uiCpBlA+l\nPmjlNRY3sHS8ylYfRHTpYBRf+MKVBqMmVSpV+VZpfmPpFwMVZIZY8DeYSWOMur1V\nYgyaVDJXJhvLGG4pLZ8rZn7TBYE19BY=\n-----END CERTIFICATE-----\n"
        }
      ]
    },
    {
      "name": "gravity-hub",
      "id": "570ffb6a-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": [
        {
          "name": "gravity.yaml",
          "content": "devmode: false\nmode: hub\npack:\n  advertise_addr: demo.gravitational.io:443\n  enabled: true\n  public_advertise_addr: demo.gravitational.io:443\nusers: null\n"
        },
        {
          "name": "teleport.yaml",
          "content": "auth_service:\n  enabled: \"true\"\n  session_recording: \"\"\nproxy_service:\n  enabled: \"true\"\n"
        }
      ]
    },
    {
      "name": "gravity-site",
      "id": "a9321cad-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": [
        {
          "name": "gravity.yaml",
          "content": "backend_type: etcd\ndata_dir: /var/lib/gravity/site\ndevmode: true\netcd:\n  key: /gravity/local\n  nodes:\n  - https://127.0.0.1:2379\n  tls_ca_file: /var/lib/gravity/secrets/root.cert\n  tls_cert_file: /var/lib/gravity/secrets/etcd.cert\n  tls_key_file: /var/lib/gravity/secrets/etcd.key\nhealth_addr: 0.0.0.0:3010\nhostname: leader.telekube.local\nops:\n  enabled: true\npack:\n  advertise_addr: leader.telekube.local:3009\n  enabled: true\n  listen_addr: 0.0.0.0:3009\n  public_listen_addr: 0.0.0.0:3007\n"
        },
        {
          "name": "teleport.yaml",
          "content": "auth_servers:\n- 127.0.0.1:3025\nauth_service:\n  cluster_name: apiserver\n  enabled: true\nproxy_service:\n  enabled: true\nteleport:\n  data_dir: /var/lib/gravity/site/teleport\n"
        }
      ]
    },
    {
      "name": "ingress-uid",
      "id": "571d585b-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": [
        {
          "name": "provider-uid",
          "content": "7a6334885f19db73"
        },
        {
          "name": "uid",
          "content": "7a6334885f19db73"
        }
      ]
    },
    {
      "name": "kube-dns",
      "id": "57554a4e-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": [
        {
          "name": "upstreamNameservers",
          "content": "[\"169.254.169.254\"]"
        }
      ]
    },
    {
      "name": "log-forwarders",
      "id": "7aacd946-0407-11e9-a200-42010a800006",
      "namespace": "kube-system",
      "created": "2018-12-20T03:29:59Z",
      "data": []
    }
  ]
}

