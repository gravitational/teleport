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
  ResourcesResponse,
  UnifiedResource,
} from 'teleport/services/agents/types';

export const fetchUnifiedResources = (opts?: {
  response?: ResourcesResponse<UnifiedResource>;
}) => {
  const {
    response = {
      startKey: 'next',
      totalCount: 12,
      items: [
        {
          kind: 'kube_cluster',
          name: 'kube-lon-dev-01.example.com',
          labels: [
            {
              name: 'env',
              value: 'dev',
            },
            {
              name: 'region',
              value: 'eu-west-1',
            },
            {
              name: 'foo',
              value: 'bar',
            },
            {
              name: 'hello',
              value: 'world',
            },
          ],
        },
        {
          kind: 'kube_cluster',
          name: 'kube-lon-prod-01.example.com',
          labels: [
            {
              name: 'env',
              value: 'prod',
            },
            {
              name: 'region',
              value: 'eu-west-1',
            },
          ],
        },
        {
          kind: 'kube_cluster',
          name: 'kube-lon-staging-01.example.com',
          labels: [
            {
              name: 'env',
              value: 'staging',
            },
            {
              name: 'region',
              value: 'eu-west-1',
            },
          ],
        },
      ],
    },
  } = opts ?? {};

  return http.get('/v1/webapi/sites/:clusterId/resources', () => {
    return HttpResponse.json(response);
  });
};
