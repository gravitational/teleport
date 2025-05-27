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

import {
  GetBotInstanceResponse,
  ListBotInstancesResponse,
} from 'teleport/services/bot/types';

const listBotInstancesPath =
  '/v1/webapi/sites/:cluster_id/machine-id/bot-instance';

const getBotInstancePath =
  '/v1/webapi/sites/:cluster_id/machine-id/bot/:bot_name/bot-instance/:id';

export const listBotInstancesSuccess = (mock: ListBotInstancesResponse) =>
  http.get(listBotInstancesPath, () => {
    return HttpResponse.json(mock);
  });

export const listBotInstancesError = (status: number) =>
  http.get(listBotInstancesPath, () => {
    return new HttpResponse(null, { status });
  });

export const getBotInstanceSuccess = (mock: GetBotInstanceResponse) =>
  http.get(getBotInstancePath, () => {
    return HttpResponse.json(mock);
  });

export const getBotInstanceError = (status: number) =>
  http.get(getBotInstancePath, () => {
    return new HttpResponse(null, { status });
  });
