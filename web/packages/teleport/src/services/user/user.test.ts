/**
 * Copyright 2020 Gravitational, Inc.
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

import user from './user';
import api from 'teleport/services/api';
import { defaultAccess } from 'teleport/services/user/makeAcl';
import { defaultStrategy } from 'teleport/services/user/makeUserContext';

test('fetch user context, null response gives proper default values', async () => {
  const mockContext = {
    authType: 'local',
    userName: 'foo',
    cluster: {
      name: 'aws',
      lastConnected: '2020-09-26T17:30:23.512876876Z',
      status: 'online',
      nodeCount: 1,
      publicURL: 'localhost',
      authVersion: '4.4.0-dev',
      proxyVersion: '4.4.0-dev',
    },
    userAcl: {
      authConnectors: {
        list: true,
        read: true,
        edit: true,
        create: true,
        remove: true,
      },
    },
  };

  jest.spyOn(api, 'get').mockResolvedValue(mockContext);

  let response = await user.fetchUserContext(false);

  expect(response.authType).toEqual('local');
  expect(response.username).toEqual('foo');
  expect(response.cluster).toMatchObject({
    clusterId: mockContext.cluster.name,
  });
  expect(response.acl).toStrictEqual({
    logins: [],
    authConnectors: mockContext.userAcl.authConnectors,
    trustedClusters: defaultAccess,
    roles: defaultAccess,
    sessions: defaultAccess,
    events: defaultAccess,
    users: defaultAccess,
    appServers: defaultAccess,
    tokens: defaultAccess,
    accessRequests: defaultAccess,
    billing: defaultAccess,
  });

  expect(response.accessStrategy).toStrictEqual(defaultStrategy);
  expect(response.requestableRoles).toStrictEqual([]);
});

test('fetch users, null response gives empty array', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);

  let response = await user.fetchUsers();

  expect(response).toStrictEqual([]);
});
