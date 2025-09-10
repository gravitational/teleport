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
import { ListWorkloadIdentitiesResponse } from 'teleport/services/workloadIdentity/types';
import { JsonObject } from 'teleport/types';

export const listWorkloadIdentitiesSuccess = (
  mock: ListWorkloadIdentitiesResponse = {
    items: [
      {
        name: 'test-workload-identity-1',
        spiffe_id: '/test/spiffe/abb53fc8-eba6-40a9-8801-221db41f3c21',
        spiffe_hint:
          'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
        labels: {
          'test-label-1': 'test-value-1',
          'test-label-2': 'test-value-2',
          'test-label-3': 'test-value-3',
        },
      },
      {
        name: 'test-workload-identity-2',
        spiffe_id: '',
        spiffe_hint: 'This is a hint',
        labels: {},
      },
      {
        name: 'test-workload-identity-3',
        spiffe_id: '/test/spiffe/6bfd8c2d-83eb-4a6f-97ba-f8b187f08339',
        spiffe_hint: '',
        labels: { 'test-label-4': 'test-value-4' },
      },
    ],
    next_page_token: 'page-token-1',
  }
) =>
  http.get(cfg.api.workloadIdentity.list, () => {
    return HttpResponse.json(mock);
  });

export const listWorkloadIdentitiesForever = () =>
  http.get(
    cfg.api.workloadIdentity.list,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );

export const listWorkloadIdentitiesError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.get(cfg.api.workloadIdentity.list, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });
