/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';

import ssh from './session';

test('fetch active sessions, response formatting', async () => {
  const sessionsJSON = [
    {
      kind: 'ssh',
      id: 'f5a5e049-b7c1-4230-85a8-65b835648fe4',
      namespace: 'default',
      parties: [
        {
          id: '36ee9829-e651-4693-b898-e638ec4f1e12',
          remote_addr: '',
          user: 'lisa2',
          server_id: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
          last_active: '2022-07-11T19:52:43.73848089Z',
        },
      ],
      terminal_params: {
        w: 80,
        h: 25,
      },
      login: 'root',
      created: '2022-07-11T19:52:43.73324983Z',
      last_active: '2022-07-11T19:52:43.73848089Z',
      server_id: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
      server_hostname: 'im-a-nodename',
      server_addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
      cluster_name: 'im-a-cluster-name',
      kubernetes_cluster_name: '',
    },
  ];

  jest.spyOn(api, 'get').mockResolvedValue({ sessions: sessionsJSON });
  const response = await ssh.fetchSessions('foo');

  expect(response[0]).toEqual(
    expect.objectContaining({
      addr: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
      clusterId: 'im-a-cluster-name',
      created: new Date('2022-07-11T19:52:43.733Z'),
      resourceName: 'im-a-nodename',
      kind: 'ssh',
      login: 'root',
      namespace: 'default',
      parties: [
        {
          user: 'lisa2',
        },
      ],
      serverId: 'd5d6d695-97c5-4bef-b052-0f5c6203d7a1',
      sid: 'f5a5e049-b7c1-4230-85a8-65b835648fe4',
    })
  );
});

test('fetch active sessions, null responses', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);
  let response = await ssh.fetchSessions('foo');
  expect(response).toEqual([]);

  jest.spyOn(api, 'get').mockResolvedValue({ sessions: [{}] });
  response = await ssh.fetchSessions('foo');
  expect(response[0].parties).toEqual([]);
});
