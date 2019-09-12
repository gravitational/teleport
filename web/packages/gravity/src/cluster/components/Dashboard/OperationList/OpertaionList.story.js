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

import $ from 'jQuery';
import React from 'react'
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { storiesOf } from '@storybook/react'
import { OperationList } from './OperationList'

storiesOf('Gravity/Dashboard', module)
  .add('OperationList', () => {
    const props = {
      pageSize: 10,
      operations,
      nodes,
      sessions,
      progress,
      onFetchProgress: () => $.Deferred().resolve(),
      onRefresh: () => $.Deferred().resolve()
    }

    return (
      <Router history={createMemoryHistory()}>
        <OperationList {...props} />
      </Router>
    );
  });

const progress = {
  "932255f0-de37-43f0-b246-dae2f546a878": {
    step: 8,
    message: 'downloading binaries'
  }
}

const operations = [
  {
    "id": "932255f0-de37-43f0-b246-dae2f546a878",
    "type": "operation_update",
    "update":{
      "update_package": "gravitational.io/hub:6.0.0-beta.1.14"
    },
    "created": "2019-05-29T21:09:00.703Z",
    "description": "Updating to gravitational.io/hub:6.0.0-beta.1.15",
    "status": "processing",
    "createdBy": "ancan@opusosi.ci",

  },
  {
    "id": "232255f0-de37-43f0-b246-dae2f546a872",
    "type": "operation_update",
    "update":{
      "update_package": "gravitational.io/hub:6.0.0-beta.1.14"
    },
    "created": "2019-05-29T21:09:00.703Z",
    "description": "Updating to gravitational.io/hub:6.0.0-beta.1.15",
    "status": "completed",
    "createdBy": "ancan@opusosi.ci",

  },
  {
    "id": "5b3969ee-4786-42e7-8a83-6a1f9cfff87a",
    "type": "operation_install",
    "created": "2019-05-28T21:56:09.471Z",
    "description": "Updating to gravitational.io/hub:6.0.0-beta.1.14",
    "status": "completed",
    "createdBy": "pawuj@oma.as",

  },
  {
    "id": "b94e27a3-3d39-448b-a434-ed0ed9ffb3a0",
    "type": "operation_expand",
    "created": "2019-05-28T19:11:09.568Z",
    "description": "Adding a server",
    "status": "failed",
    "createdBy": "cuw@jeel.zw",

  },
  {
    "id": "92a4ec27-e9b9-40ff-8960-efb0aadb244d",
    "type": "operation_uninstall",
    "created": "2019-05-23T20:34:48.814Z",
    "description": "Updating to gravitational.io/hub:6.0.0-beta.1.9",
    "status": "completed",
    "createdBy": "horpuisa@cejaj.az",

  },
  {
    "id": "34de01d4-f98f-4a7a-b19b-03db7854d435",
    "type": "operation_shrink",
    "created": "2019-05-23T02:15:15.921Z",
    "description": "Removing a server",
    "status": "completed",
    "createdBy": "gafiudi@ribi.net",

  },
]

const sessions = [{
  id: 'BZ',
  namespace: 'AG',
  login: 'MF',
  active: 'AZ',
  created: new Date(),
  durationText: '12 min',
  serverId: '10_128_0_6.demo.gravitational.io',
  siteId: '',
  sid: 'sid0',
  parties: [{
    user: 'hehwawe@aw.sg',
    remoteAddr: '129.232.123.132'
  },
  {
    user: 'ma@pewu.tz',
    remoteAddr: '129.232.123.132'
  }
  ]
}]


const nodes = [{
    "k8s": {
      "advertiseIp": 'Lidzajwa',
      "cpu": 'Robpaslic',
      "memory": 'Segunwa',
      "osImage": 'Nutvuub',
      "name": 'Ogagaib',
      "labels": {
        "key": "value",
      },
      "details": 'Mecabbut',
    },
    "canSsh": true,
    "sshLogins": [
      "root",
      "jazrafiba",
      "evubale",
    ],
    "publicIp": "232.232.323.232",
    "advertiseIp": "10.128.0.6",
    "hostname": "demo.gravitational.io",
    "id": "10_128_0_6.demo.gravitational.io",
    "instanceType": "n1-standard-2",
    "role": "node",
    "displayRole": "Ops Center Node"
  },
  {
    "k8s": {
      "advertiseIp": 'Acosupnoz',
      "cpu": 'Gojzine',
      "memory": 'Docatib',
      "osImage": 'Ithiro',
      "name": 'Ejeofiara',
      "labels": {
        "key": "value",
      },
      "details": 'Refdemmi',
    },
    "canSsh": true,
    "sshLogins": [
      "root"
    ],
    "publicIp": "232.232.323.232",
    "advertiseIp": "10.128.0.6",
    "hostname": "demo.gravitational.io",
    "id": "10_128_0_6.demo.gravitational.io",
    "instanceType": "projects/529920086732/machineTypes/n1-standard-2",
    "role": "node",
    "displayRole": "Ops Center Node"
  }
]