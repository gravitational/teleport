/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { delay, http, HttpResponse } from 'msw';

import { UnifiedInstancesResponse } from 'teleport/services/instances/types';

export const regularInstances = [
  {
    id: crypto.randomUUID(),
    type: 'instance' as const,
    instance: {
      name: 'ip-10-1-1-100.ec2.internal',
      version: '18.2.4',
      services: ['node', 'proxy'],
      upgrader: {
        type: 'systemd-unit-updater',
        version: '18.2.4',
        group: 'production',
      },
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'instance' as const,
    instance: {
      name: 'teleport-auth-01',
      version: '18.2.3',
      services: ['auth'],
      upgrader: {
        type: 'kube-updater',
        version: '18.2.3',
        group: 'staging',
      },
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'instance' as const,
    instance: {
      name: 'app-server-prod',
      version: '18.1.0',
      services: ['app', 'db'],
    },
  },
];

export const botInstances = [
  {
    id: crypto.randomUUID(),
    type: 'bot_instance' as const,
    botInstance: {
      name: 'github-actions-bot',
      version: '18.2.4',
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'bot_instance' as const,
    botInstance: {
      name: 'ci-cd-bot',
      version: '18.2.2',
    },
  },
];

export const mockInstances: UnifiedInstancesResponse = {
  instances: [...regularInstances, ...botInstances],
  startKey: '',
};

export const mockOnlyRegularInstances: UnifiedInstancesResponse = {
  instances: regularInstances,
  startKey: '',
};

export const mockOnlyBotInstances: UnifiedInstancesResponse = {
  instances: botInstances,
  startKey: '',
};

export const listInstancesSuccess = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockInstances);
  }
);

export const listOnlyRegularInstances = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockOnlyRegularInstances);
  }
);

export const listOnlyBotInstances = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockOnlyBotInstances);
  }
);

export const listInstancesError = (status: number, message: string) =>
  http.get('/v1/webapi/sites/:clusterId/instances', () => {
    return HttpResponse.json({ error: { message } }, { status });
  });

export const listInstancesLoading = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  async () => {
    await delay('infinite');
    return HttpResponse.json(mockInstances);
  }
);
