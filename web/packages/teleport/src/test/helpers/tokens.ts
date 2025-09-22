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
import { JsonObject } from 'teleport/types';

export const listV2TokensMfaError = () =>
  http.get(cfg.api.joinToken.listV2, () => {
    return HttpResponse.json(
      {
        error: { message: 'admin-level API request requires MFA verification' },
      },
      { status: 403 }
    );
  });

export const listV2TokensError = (
  status: number,
  error: string | null = null,
  fields: JsonObject = {}
) =>
  http.get(cfg.api.joinToken.listV2, () => {
    return HttpResponse.json({ error: { message: error }, fields }, { status });
  });

export const listV2TokensSuccess = (options?: {
  hasNextPage?: boolean;
  tokens?: string[];
}) => {
  const { hasNextPage = false, tokens } = options ?? {};
  return http.get(cfg.api.joinToken.listV2, () => {
    return HttpResponse.json(
      {
        items: (
          tokens ?? [
            'token',
            'ec2',
            'iam',
            'github',
            'circleci',
            'kubernetes',
            'azure',
            'gitlab',
            'gcp',
            'spacelift',
            'tpm',
            'terraform_cloud',
            'bitbucket',
            'oracle',
            'azure_devops',
            'bound_keypair',
          ]
        ).map(method => ({
          id: `token-${method}`,
          safeName: method,
          method: method,
        })),
        next_page_token: hasNextPage ? 'yes' : undefined,
      },
      { status: 200 }
    );
  });
};
