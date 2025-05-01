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
import { withUnsupportedLabelFeatureErrorConversion } from '../version/unsupported';
import makeJoinToken from './makeJoinToken';
import {
  CreateJoinTokenRequest,
  JoinRule,
  JoinToken,
  JoinTokenRequest,
} from './types';

const TeleportTokenNameHeader = 'X-Teleport-TokenName';

class JoinTokenService {
  // TODO (avatus) refactor this code to eventually use `createJoinToken`
  fetchJoinTokenV2(
    req: JoinTokenRequest,
    signal: AbortSignal = null
  ): Promise<JoinToken> {
    return (
      api
        .post(
          cfg.api.discoveryJoinToken.createV2,
          {
            roles: req.roles,
            join_method: req.method || 'token',
            allow: makeAllowField(req.rules || []),
            suggested_agent_matcher_labels: makeLabelMapOfStrArrs(
              req.suggestedAgentMatcherLabels
            ),
            suggested_labels: makeLabelMapOfStrArrs(req.suggestedLabels),
          },
          signal
        )
        .then(makeJoinToken)
        // TODO(kimlisa): DELETE IN 19.0
        .catch(withUnsupportedLabelFeatureErrorConversion)
    );
  }

  // TODO(kimlisa): DELETE IN 19.0
  // replaced by fetchJoinTokenV2 that accepts labels.
  fetchJoinToken(
    req: Omit<JoinTokenRequest, 'suggestedLabels'>,
    signal: AbortSignal = null
  ): Promise<JoinToken> {
    return api
      .post(
        cfg.api.discoveryJoinToken.create,
        {
          roles: req.roles,
          join_method: req.method || 'token',
          allow: makeAllowField(req.rules || []),
          suggested_agent_matcher_labels: makeLabelMapOfStrArrs(
            req.suggestedAgentMatcherLabels
          ),
        },
        signal
      )
      .then(makeJoinToken);
  }

  upsertJoinTokenYAML(
    req: JoinTokenRequest,
    tokenName: string
  ): Promise<JoinToken> {
    return api
      .putWithHeaders(
        cfg.getJoinTokenYamlUrl(),
        {
          content: req.content,
        },
        {
          [TeleportTokenNameHeader]: tokenName,
          'Content-Type': 'application/json',
        }
      )
      .then(makeJoinToken);
  }

  async createJoinToken(req: CreateJoinTokenRequest) {
    return api.post(cfg.getJoinTokensUrl(), req).then(makeJoinToken);
  }

  async editJoinToken(req: CreateJoinTokenRequest) {
    const json = await api.put(cfg.getJoinTokensUrl(), req);
    return makeJoinToken(json);
  }

  fetchJoinTokens(signal: AbortSignal = null): Promise<{ items: JoinToken[] }> {
    return api.get(cfg.getJoinTokensUrl(), signal).then(resp => {
      return {
        items: resp.items?.map(makeJoinToken) || [],
      };
    });
  }

  deleteJoinToken(id: string, signal: AbortSignal = null) {
    return api.deleteWithHeaders(
      cfg.getJoinTokensUrl(),
      { [TeleportTokenNameHeader]: id },
      signal
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
