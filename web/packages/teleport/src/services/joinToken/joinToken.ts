/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import makeJoinToken from './makeJoinToken';
import { JoinToken, JoinMethod, JoinRole, JoinRule } from './types';

class JoinTokenService {
  fetchJoinToken(
    // roles is a list of join roles, since there can be more than
    // one role associated with a token.
    roles: JoinRole[],
    joinMethod: JoinMethod = 'token',
    // rules is a list of allow rules associated with the join token
    // and the node using this token must match one of the rules.
    rules: JoinRule[] = [],
    signal: AbortSignal = null
  ): Promise<JoinToken> {
    return api
      .post(
        cfg.getJoinTokenUrl(),
        {
          roles,
          join_method: joinMethod,
          allow: makeAllowField(rules),
        },
        signal
      )
      .then(makeJoinToken);
  }
}

function makeAllowField(rules: JoinRule[]) {
  return rules.map(rule => ({
    aws_account: rule.awsAccountId,
    aws_arn: rule.awsArn,
  }));
}

export default JoinTokenService;
