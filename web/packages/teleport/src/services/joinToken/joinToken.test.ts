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

import JoinTokenService from './joinToken';
import type { JoinTokenRequest } from './types';

beforeEach(() => {
  jest.resetAllMocks();
});

test('fetchJoinToken with an empty request properly sets defaults', () => {
  const svc = new JoinTokenService();
  jest.spyOn(api, 'post').mockResolvedValue(null);

  // Test with all empty fields.
  svc.fetchJoinTokenV2({} as any);
  expect(api.post).toHaveBeenCalledWith(
    cfg.api.discoveryJoinToken.createV2,
    {
      roles: undefined,
      join_method: 'token',
      allow: [],
      suggested_agent_matcher_labels: {},
      suggested_labels: {},
    },
    null
  );
});

test('fetchJoinToken request fields are set as requested', () => {
  const svc = new JoinTokenService();
  jest.spyOn(api, 'post').mockResolvedValue(null);

  const mock: JoinTokenRequest = {
    roles: ['Node'],
    rules: [{ awsAccountId: '1234', awsArn: 'xxxx' }],
    method: 'iam',
    suggestedAgentMatcherLabels: [{ name: 'env', value: 'dev' }],
  };
  svc.fetchJoinTokenV2(mock);
  expect(api.post).toHaveBeenCalledWith(
    cfg.api.discoveryJoinToken.createV2,
    {
      roles: ['Node'],
      join_method: 'iam',
      allow: [{ aws_account: '1234', aws_arn: 'xxxx' }],
      suggested_agent_matcher_labels: { env: ['dev'] },
      suggested_labels: {},
    },
    null
  );
});
