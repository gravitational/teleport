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

import { makeLabelMapOfStrArrs } from '../agents/make';
import auth from '../auth/auth';

import makeJoinToken from './makeJoinToken';
import { JoinRule, JoinToken, JoinTokenRequest } from './types';

const TeleportTokenNameHeader = 'X-Teleport-TokenName';

class JoinTokenService {
  // TODO (avatus) refactor this code to eventually use `createJoinToken`
  async fetchJoinToken(
    req: JoinTokenRequest,
    signal: AbortSignal = null
  ): Promise<JoinToken> {
    const mfaResponse = await auth.getAdminActionMfaResponse();
    return api
      .post(
        cfg.getJoinTokenUrl(),
        {
          roles: req.roles,
          join_method: req.method || 'token',
          allow: makeAllowField(req.rules || []),
          suggested_agent_matcher_labels: makeLabelMapOfStrArrs(
            req.suggestedAgentMatcherLabels
          ),
        },
        signal,
        mfaResponse
      )
      .then(makeJoinToken);
  }

  async upsertJoinTokenYAML(
    req: JoinTokenRequest,
    tokenName: string
  ): Promise<JoinToken> {
    const mfaResponse = await auth.getAdminActionMfaResponse();
    return api
      .putWithHeaders(
        cfg.getJoinTokenYamlUrl(),
        {
          content: req.content,
        },
        {
          [TeleportTokenNameHeader]: tokenName,
          'Content-Type': 'application/json',
        },
        mfaResponse
      )
      .then(makeJoinToken);
  }

  async createJoinToken(req: JoinTokenRequest): Promise<JoinToken> {
    const mfaResponse = await auth.getAdminActionMfaResponse();
    return api
      .post(cfg.getJoinTokensUrl(), req, mfaResponse)
      .then(makeJoinToken);
  }

  async fetchJoinTokens(
    signal: AbortSignal = null
  ): Promise<{ items: JoinToken[] }> {
    const mfaResponse = await auth.getAdminActionMfaResponse();
    return api.get(cfg.getJoinTokensUrl(), signal, mfaResponse).then(resp => {
      return {
        items: resp.items?.map(makeJoinToken) || [],
      };
    });
  }

  async deleteJoinToken(id: string, signal: AbortSignal = null) {
    const mfaResponse = await auth.getAdminActionMfaResponse();
    return api.deleteWithHeaders(
      cfg.getJoinTokensUrl(),
      { [TeleportTokenNameHeader]: id },
      signal,
      mfaResponse
    );
  }
}

function makeAllowField(rules: JoinRule[] = []) {
  return rules.map(rule => ({
    aws_account: rule.awsAccountId,
    aws_arn: rule.awsArn,
  }));
}

export default JoinTokenService;
