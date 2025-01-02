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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import { agentService } from './agents';
import { makeLabelMapOfStrArrs } from './make';
import type { ConnectionDiagnosticRequest } from './types';

test('createConnectionDiagnostic request', () => {
  jest.spyOn(api, 'post').mockResolvedValue(null);

  // Test with all empty fields.
  agentService.createConnectionDiagnostic({} as any);
  expect(api.post).toHaveBeenCalledWith(cfg.getConnectionDiagnosticUrl(), {
    resource_kind: undefined,
    resource_name: undefined,
    ssh_principal: undefined,
    kubernetes_namespace: undefined,
    kubernetes_impersonation: {
      kubernetes_user: undefined,
      kubernetes_groups: undefined,
    },
    database_name: undefined,
    database_user: undefined,
  });

  // Test all fields gets set as requested.
  const mock: ConnectionDiagnosticRequest = {
    resourceKind: 'node',
    resourceName: 'resource_name',
    sshPrincipal: 'ssh_principal',
    kubeImpersonation: {
      namespace: 'kubernetes_namespace',
      user: 'kubernetes_user',
      groups: ['group1', 'group2'],
    },
    dbTester: {
      name: 'db_name',
      user: 'db_user',
    },
  };
  agentService.createConnectionDiagnostic(mock);
  expect(api.post).toHaveBeenCalledWith(cfg.getConnectionDiagnosticUrl(), {
    resource_kind: 'node',
    resource_name: 'resource_name',
    ssh_principal: 'ssh_principal',
    kubernetes_namespace: 'kubernetes_namespace',
    kubernetes_impersonation: {
      kubernetes_user: 'kubernetes_user',
      kubernetes_groups: ['group1', 'group2'],
    },
    database_name: 'db_name',
    database_user: 'db_user',
  });
});

test('correct makeLabelMapOfStrArrs', () => {
  // Test empty param.
  let result = makeLabelMapOfStrArrs();
  expect(result).toStrictEqual({});

  // Test with param.
  result = makeLabelMapOfStrArrs([
    { name: 'os', value: 'mac' },
    { name: 'os', value: 'linux' },
    { name: 'env', value: 'prod' },
  ]);
  expect(result).toStrictEqual({ os: ['mac', 'linux'], env: ['prod'] });
});
