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

import AuditService from './audit';

test('fetch events', async () => {
  const audit = new AuditService();

  // Test null response gives empty array.
  jest.spyOn(api, 'get').mockResolvedValue({ events: null });
  let response = await audit.fetchEvents('clusterId', params);

  expect(api.get).toHaveBeenCalledTimes(1);
  expect(response.events).toEqual([]);
  expect(response.startKey).toBeUndefined();

  // Test normal response.
  audit.maxFetchLimit = 2;
  jest.spyOn(api, 'get').mockResolvedValue(normalJson);
  response = await audit.fetchEvents('clusterId', params);

  expect(response.startKey).toEqual(normalJson.startKey);
  expect(response.events).toEqual([
    {
      codeDesc: 'Reset Password Token Created',
      message:
        'User [90678c66-ffcc-4f02.im-a-cluster-name] created a password reset token for user [root]',
      id: '5ec6-4c2c-8567-36bcb',
      code: 'T6000I',
      user: '90678c66-ffcc-4f02.im-a-cluster-name',
      time: new Date('2021-05-25T07:34:22.204Z'),
      raw: {
        cluster_name: 'im-a-cluster-name',
        code: 'T6000I',
        ei: 0,
        event: 'reset_password_token.create',
        expires: '2021-05-25T08:34:22.204114385Z',
        name: 'root',
        time: '2021-05-25T07:34:22.204Z',
        ttl: '1h0m0s',
        uid: '5ec6-4c2c-8567-36bcb',
        user: '90678c66-ffcc-4f02.im-a-cluster-name',
      },
    },
    // Test without uid, id field returns event:time format
    {
      codeDesc: 'Local Login',
      message: 'Local user [root] successfully logged in',
      id: 'user.login:2021-05-25T14:37:27.848Z',
      code: 'T1000I',
      user: 'root',
      time: new Date('2021-05-25T14:37:27.848Z'),
      raw: {
        cluster_name: 'im-a-cluster-name',
        code: 'T1000I',
        ei: 0,
        event: 'user.login',
        method: 'local',
        success: true,
        time: '2021-05-25T14:37:27.848Z',
        user: 'root',
      },
    },
  ]);

  // Test unknown event code returns unknown format
  jest.spyOn(api, 'get').mockResolvedValue(unknownEvent);
  response = await audit.fetchEvents('clusterId', params);

  expect(response.events[0].codeDesc).toBe('Unknown');
  expect(response.events[0].message).toBe('Unknown');
});

const params = {
  from: new Date(0),
  to: new Date(0),
};

const normalJson = {
  events: [
    {
      cluster_name: 'im-a-cluster-name',
      code: 'T6000I',
      ei: 0,
      event: 'reset_password_token.create',
      expires: '2021-05-25T08:34:22.204114385Z',
      name: 'root',
      time: '2021-05-25T07:34:22.204Z',
      ttl: '1h0m0s',
      uid: '5ec6-4c2c-8567-36bcb',
      user: '90678c66-ffcc-4f02.im-a-cluster-name',
    },
    {
      cluster_name: 'im-a-cluster-name',
      code: 'T1000I',
      ei: 0,
      event: 'user.login',
      method: 'local',
      success: true,
      time: '2021-05-25T14:37:27.848Z',
      user: 'root',
    },
  ],
  startKey: '0691-4797-ab2b-8c7b8',
};

const unknownEvent = {
  events: [
    {
      cluster_name: 'im-a-cluster-name',
      code: 'unregistered-code',
      ei: 0,
      event: 'user.login',
      method: 'local',
      success: true,
      time: '2021-05-25T14:37:27.848Z',
      user: 'root',
    },
  ],
};
