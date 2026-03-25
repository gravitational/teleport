/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import {
  isResourceHealthStatus,
  ResourceHealthStatus,
  UnifiedResourceApp,
  UnifiedResourceDatabase,
  UnifiedResourceDesktop,
  UnifiedResourceGitServer,
  UnifiedResourceKube,
  UnifiedResourceNode,
  UnifiedResourceUserGroup,
} from './types';
import { generateUnifiedResourceKey } from './UnifiedResources';

test('isResourceStatus', () => {
  const validStatuses: ResourceHealthStatus[] = [
    'healthy',
    'unhealthy',
    'unknown',
    'mixed',
  ];
  const statuses = [
    'healthy',
    'random',
    'unhealthy',
    'llama',
    'unknown',
    '',
    'mixed',
  ];

  expect(statuses.filter(isResourceHealthStatus)).toEqual(validStatuses);
});

describe('generateUnifiedResourceKey', () => {
  test('generates key for node resource using hostname/id/kind format', () => {
    const nodeResource: UnifiedResourceNode = {
      kind: 'node',
      hostname: 'MyServer',
      id: 'abc-123',
      addr: '127.0.0.1',
      tunnel: false,
      subKind: 'teleport',
      labels: [],
    };

    expect(generateUnifiedResourceKey(nodeResource)).toBe(
      'myserver/abc-123/node'
    );
  });

  test('generates key for git_server resource using hostname/id/kind format', () => {
    const gitServerResource: UnifiedResourceGitServer = {
      kind: 'git_server',
      hostname: 'GitServer',
      id: 'git-456',
      labels: [],
      subKind: 'github',
      github: {
        organization: 'my-org',
        integration: 'my-integration',
      },
    };

    expect(generateUnifiedResourceKey(gitServerResource)).toBe(
      'gitserver/git-456/git_server'
    );
  });

  test('generates key for app resource with friendlyName using friendlyName/name/kind format', () => {
    const appResource: UnifiedResourceApp = {
      kind: 'app',
      id: 'app-123',
      name: 'my-app',
      friendlyName: 'MyFriendlyApp',
      description: 'Test app',
      labels: [],
      awsConsole: false,
      samlApp: false,
    };

    expect(generateUnifiedResourceKey(appResource)).toBe(
      'myfriendlyapp/my-app/app'
    );
  });

  test('generates key for app resource without friendlyName using name/kind format', () => {
    const appResource: UnifiedResourceApp = {
      kind: 'app',
      id: 'app-124',
      name: 'my-app',
      friendlyName: '',
      description: 'Test app',
      labels: [],
      awsConsole: false,
      samlApp: false,
    };

    expect(generateUnifiedResourceKey(appResource)).toBe('my-app/app');
  });

  test('generates key for database resource using name/kind format', () => {
    const dbResource: UnifiedResourceDatabase = {
      kind: 'db',
      name: 'MyDatabase',
      description: 'Test database',
      type: 'postgres',
      protocol: 'postgres',
      labels: [],
    };

    expect(generateUnifiedResourceKey(dbResource)).toBe('mydatabase/db');
  });

  test('generates key for kube_cluster resource using name/kind format', () => {
    const kubeResource: UnifiedResourceKube = {
      kind: 'kube_cluster',
      name: 'MyKubeCluster',
      labels: [],
    };

    expect(generateUnifiedResourceKey(kubeResource)).toBe(
      'mykubecluster/kube_cluster'
    );
  });

  test('generates key for windows_desktop resource using name/kind format', () => {
    const desktopResource: UnifiedResourceDesktop = {
      kind: 'windows_desktop',
      name: 'MyDesktop',
      os: 'windows',
      addr: '127.0.0.1',
      labels: [],
    };

    expect(generateUnifiedResourceKey(desktopResource)).toBe(
      'mydesktop/windows_desktop'
    );
  });

  test('generates key for user_group resource using name/kind format', () => {
    const userGroupResource: UnifiedResourceUserGroup = {
      kind: 'user_group',
      name: 'MyUserGroup',
      description: 'Test group',
      labels: [],
    };

    expect(generateUnifiedResourceKey(userGroupResource)).toBe(
      'myusergroup/user_group'
    );
  });
});
