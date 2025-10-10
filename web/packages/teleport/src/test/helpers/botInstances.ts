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

import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import {
  BotInstanceKind,
  BotInstanceServiceHealthStatus,
  GetBotInstanceResponse,
  ListBotInstancesResponse,
} from 'teleport/services/bot/types';

export const listBotInstancesSuccess = (mock: ListBotInstancesResponse) =>
  http.get(cfg.api.botInstance.list, () => {
    return HttpResponse.json(mock);
  });

export const listBotInstancesForever = () =>
  http.get(
    cfg.api.botInstance.list,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const listBotInstancesError = (
  status: number,
  error: string | null = null
) =>
  http.get(cfg.api.botInstance.list, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

export const getBotInstanceSuccess = (mock?: GetBotInstanceResponse) =>
  http.get(cfg.api.botInstance.read, () => {
    return HttpResponse.json(mock ?? mockGetBotInstanceResponse);
  });

export const getBotInstanceError = (
  status: number,
  error: string | null = null
) =>
  http.get(cfg.api.botInstance.read, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

export const getBotInstanceForever = () =>
  http.get(
    cfg.api.botInstance.read,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const mockGetBotInstanceResponse = {
  bot_instance: {
    spec: {
      instance_id: 'a55259e8-9b17-466f-9d37-ab390ca4024e',
      bot_name: 'test-bot-name',
    },
    status: {
      latest_heartbeats: [
        {
          uptime: {
            seconds: 43200 + 60,
          },
          version: '18.4.0',
          hostname: 'test-hostname',
          os: 'linux',
          kind: BotInstanceKind.BOT_KIND_TBOT_BINARY,
        },
      ],
      latest_authentications: [
        {
          metadata: {
            join_method: 'github',
            join_token_name: 'test-token-name',
          },
          join_attrs: {
            github: {
              sub: 'test-github-sub',
              repository: 'gravitational/teleport',
            },
          },
        },
      ],
      service_health: [
        {
          service: {
            name: 'application-tunnel-1',
            type: 'application-tunnel',
          },
          status:
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_HEALTHY,
          updated_at: {
            seconds: new Date('2025-10-10T10:45:00Z').getTime() / 1_000,
          },
        },
        {
          service: {
            name: 'db-eu-lon-1',
            type: 'database-tunnel',
          },
          status:
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY,
          updated_at: {
            seconds: new Date('2025-10-10T10:46:00Z').getTime() / 1_000,
          },
          reason:
            'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
        },
        {
          service: {
            name: 'workload-identity-aws-roles-anywhere-1',
            type: 'workload-identity-aws-roles-anywhere',
          },
          status:
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_INITIALIZING,
          updated_at: {
            seconds: new Date('2025-10-10T10:47:00Z').getTime() / 1_000,
          },
        },
        {
          service: {
            name: 'application-tunnel-2',
            type: 'application-tunnel',
          },
          status:
            BotInstanceServiceHealthStatus.BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED,
          updated_at: {
            seconds: new Date('2025-10-10T10:48:00Z').getTime() / 1_000,
          },
        },
      ],
    },
  },
  yaml: 'kind: bot_instance\nversion: v1\n',
};
