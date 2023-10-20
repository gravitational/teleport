/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import JoinTokenService from './joinToken';

import type { JoinTokenRequest } from './types';

test('fetchJoinToken with an empty request properly sets defaults', () => {
  const svc = new JoinTokenService();
  jest.spyOn(api, 'post').mockResolvedValue(null);

  // Test with all empty fields.
  svc.fetchJoinToken({} as any);
  expect(api.post).toHaveBeenCalledWith(
    cfg.getJoinTokenUrl(),
    {
      roles: undefined,
      join_method: 'token',
      allow: [],
      suggested_agent_matcher_labels: {},
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
  svc.fetchJoinToken(mock);
  expect(api.post).toHaveBeenCalledWith(
    cfg.getJoinTokenUrl(),
    {
      roles: ['Node'],
      join_method: 'iam',
      allow: [{ aws_account: '1234', aws_arn: 'xxxx' }],
      suggested_agent_matcher_labels: { env: ['dev'] },
    },
    null
  );
});
