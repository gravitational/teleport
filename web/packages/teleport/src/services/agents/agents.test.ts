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
import cfg from 'teleport/config';

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
