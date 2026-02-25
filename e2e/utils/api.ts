/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Page } from '@playwright/test';

const CLUSTER_ID = 'teleport-e2e';

export async function getAuthHeaders(page: Page) {
  return page.evaluate(() => {
    const meta = document.querySelector(
      'meta[name="grv_csrf_token"]'
    ) as HTMLMetaElement;
    const token = JSON.parse(
      localStorage.getItem('grv_teleport_token') || '{}'
    );
    return {
      'X-CSRF-Token': meta?.content || '',
      Authorization: `Bearer ${token.accessToken || ''}`,
    };
  });
}

export async function createIntegration(
  page: Page,
  name: string,
  roleArn: string
) {
  const headers = await getAuthHeaders(page);
  return page.request.post(`/v1/webapi/sites/${CLUSTER_ID}/integrations`, {
    headers,
    data: {
      name,
      subKind: 'aws-oidc',
      awsoidc: { roleArn },
    },
  });
}

export async function deleteIntegration(page: Page, name: string) {
  const headers = await getAuthHeaders(page);
  return page.request.delete(
    `/v1/webapi/sites/${CLUSTER_ID}/integrations/${name}`,
    { headers }
  );
}
