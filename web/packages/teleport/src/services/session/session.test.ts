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
