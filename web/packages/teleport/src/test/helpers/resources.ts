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
import { JsonObject } from 'teleport/types';

// Using the path without query string params included
const path = '/v1/webapi/sites/:clusterId/resources';

export const fetchUnifiedResourcesSuccess = (opts?: {
  // FIXME: the response type should be raw API payload, not `UnifiedResource`.
  // `UnifiedResource` is produced later by `resourceService.fetchUnifiedResources()`
  // via `makeUnifiedResource`.
  // Mixing these layers can cause some values to be missing after going through makeUnifiedResource.
  response?: ResourcesResponse<UnifiedResource> & { items: UnifiedResource[] };
  delayMs?: number;
  mockSearch?: boolean;
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
        {
          kind: 'kube_cluster',
          name: 'kube-temp-01.example.com',
        },
      ],
    },
    delayMs = 0,
    mockSearch = false,
  } = opts ?? {};

  return http.get(path, async ({ request }) => {
    const url = new URL(request.url);
    const search = url.searchParams.get('search');

    if (delayMs) {
      await new Promise(res => setTimeout(res, delayMs));
    }

    return HttpResponse.json({
      ...response,
      ...(mockSearch && search
        ? {
            items: response.items.filter(
              a => a.kind != 'kube_cluster' || a.name.includes(search)
            ),
          }
        : {}),
    });
  });
};

export const fetchUnifiedResourcesError = (
  status: number,
  error: string | null = null,
  extras: JsonObject = {}
) =>
  http.get(path, () => {
    return HttpResponse.json(
      { error: { message: `${status} - ${error}` }, extras },
      { status }
    );
  });

export const fetchUnifiedResourcesForever = () =>
  http.get(
    path,
    () =>
      new Promise(() => {
        /* never resolved */
      })
  );
