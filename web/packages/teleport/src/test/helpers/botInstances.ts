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
  GetBotInstanceMetricsResponse,
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

export const getBotInstanceSuccess = (mock: GetBotInstanceResponse) =>
  http.get(cfg.api.botInstance.read, () => {
    return HttpResponse.json(mock);
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

export const getBotInstanceMetricsSuccess = (
  mock?: GetBotInstanceMetricsResponse
) =>
  http.get(cfg.api.botInstance.metrics, () => {
    return HttpResponse.json(
      mock ?? {
        upgrade_statuses: {
          updated_at: new Date().toISOString(),
          up_to_date: {
            count: randBetween(0, 2000),
            filter: 'up to date filter goes here',
          },
          patch_available: {
            count: randBetween(0, 2000),
            filter: 'patch filter goes here',
          },
          requires_upgrade: {
            count: randBetween(0, 2000),
            filter: 'upgrade filter goes here',
          },
          unsupported: {
            count: randBetween(0, 2000),
            filter: 'unsupported filter goes here',
          },
        },
      }
    );
  });

export const getBotInstanceMetricsForever = () =>
  http.get(
    cfg.api.botInstance.metrics,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const getBotInstanceMetricsError = (
  status: number,
  error: string | null = null
) =>
  http.get(cfg.api.botInstance.metrics, () => {
    return HttpResponse.json({ error: { message: error } }, { status });
  });

function randBetween(low: number, high: number) {
  if (low > high) [low, high] = [high, low];
  return Math.floor(Math.random() * (high - low + 1)) + low;
}
