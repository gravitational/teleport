/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import cfg from 'teleport/config';
import api from 'teleport/services/api';

import auth from '../auth/auth';

import JoinTokenService from './joinToken';

import type { JoinTokenRequest } from './types';
test('fetchJoinToken with an empty request properly sets defaults', async () => {
  const svc = new JoinTokenService();
  jest.spyOn(api, 'post').mockResolvedValue(null);
  jest.spyOn(auth, 'getAdminActionMfaResponse').mockResolvedValue(null);

  // Test with all empty fields.
  await svc.fetchJoinToken({} as any);
  expect(api.post).toHaveBeenCalledWith(
    cfg.getJoinTokenUrl(),
    {
      roles: undefined,
      join_method: 'token',
      allow: [],
      suggested_agent_matcher_labels: {},
    },
    null,
    null
  );
});

test('fetchJoinToken request fields are set as requested', async () => {
  const svc = new JoinTokenService();
  jest.spyOn(api, 'post').mockResolvedValue(null);
  jest.spyOn(auth, 'getAdminActionMfaResponse').mockResolvedValue(null);

  const mock: JoinTokenRequest = {
    roles: ['Node'],
    rules: [{ awsAccountId: '1234', awsArn: 'xxxx' }],
    method: 'iam',
    suggestedAgentMatcherLabels: [{ name: 'env', value: 'dev' }],
  };
  await svc.fetchJoinToken(mock);

  expect(api.post).toHaveBeenCalledWith(
    cfg.getJoinTokenUrl(),
    {
      roles: ['Node'],
      join_method: 'iam',
      allow: [{ aws_account: '1234', aws_arn: 'xxxx' }],
      suggested_agent_matcher_labels: { env: ['dev'] },
    },
    null,
    null
  );
});
