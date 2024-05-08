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

import user from './user';
import { makeTraits } from './makeUser';
import { PasswordState } from './types';

test('undefined values in context response gives proper default values', async () => {
  const mockContext = {
    authType: 'local',
    userName: 'foo',
    cluster: {
      name: 'aws',
      lastConnected: new Date('2020-09-26T17:30:23.512876876Z'),
      status: 'online',
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

  const response = await user.fetchUserContext(false);
  expect(response).toEqual({
    username: 'foo',
    authType: 'local',
    acl: {
      accessList: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      accessMonitoringRule: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      authConnectors: {
        list: true,
        read: true,
        edit: true,
        create: true,
        remove: true,
      },
      // Test that undefined acl booleans are set to default false.
      trustedClusters: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      nodes: {
        create: false,
        edit: false,
        list: false,
        read: false,
        remove: false,
      },
      plugins: {
        create: false,
        edit: false,
        list: false,
        read: false,
        remove: false,
      },
      integrations: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
        use: false,
      },
      roles: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      lock: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      recordedSessions: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      desktops: {
        create: false,
        edit: false,
        list: false,
        read: false,
        remove: false,
      },
      events: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      externalAuditStorage: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      users: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      activeSessions: {
        create: false,
        edit: false,
        list: false,
        read: false,
        remove: false,
      },
      appServers: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      kubeServers: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      license: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      download: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      tokens: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      accessRequests: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      billing: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      dbServers: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      db: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      connectionDiagnostic: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      deviceTrust: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      assist: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      samlIdpServiceProvider: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      auditQuery: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      securityReport: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      accessGraph: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      bots: {
        list: false,
        read: false,
        edit: false,
        create: false,
        remove: false,
      },
      clipboardSharingEnabled: true,
      desktopSessionRecordingEnabled: true,
      directorySharingEnabled: true,
    },
    cluster: {
      clusterId: 'aws',
      lastConnected: new Date('2020-09-26T17:30:23.512Z'),
      connectedText: '2020-09-26 17:30:23',
      status: 'online',
      url: '/web/cluster/aws/',
      authVersion: '4.4.0-dev',
      publicURL: 'localhost',
      proxyVersion: '4.4.0-dev',
    },
    // Test undefined access strategy is set to default optional.
    accessStrategy: { type: 'optional', prompt: '' },
    // Test undefined roles and reviewers are set to empty arrays.
    accessCapabilities: { requestableRoles: [], suggestedReviewers: [] },
    allowedSearchAsRoles: [],
    passwordState: PasswordState.PASSWORD_STATE_UNSPECIFIED,
  });
});

test('fetch users, null response values gives empty array', async () => {
  jest.spyOn(api, 'get').mockResolvedValue(null);
  let response = await user.fetchUsers();
  expect(response).toStrictEqual([]);

  jest.spyOn(api, 'get').mockResolvedValue([{ name: '', authType: '' }]);

  response = await user.fetchUsers();
  expect(response).toStrictEqual([
    {
      authType: '',
      isLocal: false,
      name: '',
      roles: [],
      allTraits: {},
      traits: {
        awsRoleArns: [],
        databaseNames: [],
        databaseUsers: [],
        kubeGroups: [],
        kubeUsers: [],
        logins: [],
        windowsLogins: [],
      },
    },
  ]);
});

test('createResetPasswordToken', async () => {
  // Test null response.
  jest.spyOn(api, 'post').mockResolvedValue(null);
  let response = await user.createResetPasswordToken('name', 'invite');
  expect(response).toStrictEqual({
    username: '',
    expires: null,
    value: '',
  });

  // Test with a valid response.
  jest.spyOn(api, 'post').mockResolvedValue({
    expiry: 1677273148317,
    user: 'llama',
    tokenId: 'some-id',
  });
  response = await user.createResetPasswordToken('name', 'invite');
  expect(response).toStrictEqual({
    username: 'llama',
    expires: new Date(1677273148317),
    value: 'some-id',
  });
});

test('makeTraits', async () => {
  expect(makeTraits(null)).toStrictEqual({});
  expect(makeTraits({})).toStrictEqual({});

  const mockTraits = {
    fruit: null,
    drink: [],
    pet: [''],
    movie: null,
    holiday: ['halloween', 'christmas'],
    color: null,
  };

  expect(makeTraits(mockTraits)).toStrictEqual({
    fruit: [],
    drink: [],
    pet: [''],
    movie: [],
    holiday: ['halloween', 'christmas'],
    color: [],
  });
});
