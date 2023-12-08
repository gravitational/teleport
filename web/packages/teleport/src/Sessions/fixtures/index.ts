/**
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
    command: 'kubectl get pods',
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
    command: 'ls -la',
  },
  {
    kind: 'ssh',
    sid: 'b204924e-6b74-5d92-89ea-d95043a969f1',
    namespace: 'default',
    parties: [
      {
        user: 'lisa3',
      },
    ],
    login: 'root',
    created: new Date('2022-07-11T14:36:14.491402068Z'),
    durationText: '5 seconds',
    serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    resourceName: 'im-a-nodename-2',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer', 'moderator', 'peer'],
    moderated: false,
    command:
      'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
  },
  {
    kind: 'ssh',
    sid: '8830cfe5-369e-5485-9c3d-19cc50e6f548',
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
    resourceName: 'im-a-nodename-3',
    addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
    clusterId: 'im-a-cluster-name',
    participantModes: ['observer'],
    moderated: false,
    command:
      'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
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
    command:
      'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
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
    command:
      'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
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
    command:
      'top -o command -o cpu -o boosts -o cycles -o cow -o user -o vsize -o csw -o threads -o ports -o ppid',
  },
];
