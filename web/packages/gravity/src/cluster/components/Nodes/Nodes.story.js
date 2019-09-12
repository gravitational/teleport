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

import React from 'react';
import $ from 'jQuery';
import { storiesOf } from '@storybook/react'
import { Nodes } from './Nodes'

storiesOf('Gravity/Nodes', module)
  .add('Nodes', () => {
    return (
      <Nodes onFetch={ () => $.Deferred() } nodes={nodes} />
      );
  });

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
