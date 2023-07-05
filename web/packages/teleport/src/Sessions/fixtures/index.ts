/*
Copyright 2020-2022 Gravitational, Inc.

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

import { Session } from 'teleport/services/session';

export const sessions: Session[] = [
  {
    kind: 'k8s',
    sid: '7174aded-340a-4863-b661-7ba52aeb22c8',
    namespace: 'default',
    parties: [
      {
        user: 'lisa2',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T15:34:33.256697813Z'),
    durationText: '59 minutes',
    addr: '',
    serverId: '',
    clusterId: 'im-a-cluster-name',
    resourceName: 'minikube',
    participantModes: ['observer', 'moderator', 'peer'],
    moderated: false,
  },
  {
    kind: 'ssh',
    sid: 'c7befbb4-3885-4d08-a466-de832a73c3d4',
    namespace: 'default',
    parties: [
      {
        user: 'lisa2',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T14:36:14.491402068Z'),
    durationText: '5 seconds',
    serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    resourceName: 'im-a-nodename',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer', 'moderator'],
    moderated: false,
  },
  {
    kind: 'desktop',
    sid: 'acacfbb4-3885-4d08-a466-de832a73ffac',
    namespace: 'default',
    parties: [
      {
        user: 'lisa2',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T14:36:14.491402068Z'),
    durationText: '5 seconds',
    serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    resourceName: 'desktop-2',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer', 'moderator', 'peer'],
    moderated: false,
  },
  {
    kind: 'db',
    sid: '2314fbb4-3885-4d08-a466-de832a731222',
    namespace: 'default',
    parties: [
      {
        user: 'lisa2',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T14:36:14.491402068Z'),
    durationText: '3 minutes',
    serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    resourceName: 'databse-32',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer'],
    moderated: false,
  },
  {
    kind: 'app',
    sid: 'cafefbb4-3885-4d08-a466-de832a7313131',
    namespace: 'default',
    parties: [
      {
        user: 'lisa2',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T14:36:14.491402068Z'),
    durationText: '13 minutes',
    serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    resourceName: 'grafana',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer', 'moderator', 'peer'],
    moderated: false,
  },
];
