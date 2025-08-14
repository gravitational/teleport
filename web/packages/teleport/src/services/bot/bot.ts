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
import {
  canUseV1Edit,
  makeBot,
  toApiGitHubTokenSpec,
  validateGetBotInstanceResponse,
  validateListBotInstancesResponse,
} from 'teleport/services/bot/consts';
import ResourceService, { RoleResource } from 'teleport/services/resources';
import { FeatureFlags } from 'teleport/types';

import { validateListJoinTokensResponse } from '../joinToken/consts';
import { MfaChallengeResponse } from '../mfa';
import { withGenericUnsupportedError } from '../version/unsupported';
import {
  BotResponse,
  CreateBotJoinTokenRequest,
  CreateBotRequest,
  EditBotRequest,
  FlatBot,
} from './types';

export function createBot(
  config: CreateBotRequest,
  mfaResponse?: MfaChallengeResponse
): Promise<void> {
  return api.post(
    cfg.getBotUrl({ action: 'create' }),
    config,
    undefined /* abort signal */,
    mfaResponse
  );
}

export async function getBot(
  variables: {
    botName: string;
  },
  signal?: AbortSignal
): Promise<FlatBot | null> {
  try {
    return await api
      .get(cfg.getBotUrl({ action: 'read', ...variables }), signal)
      .then(makeBot);
  } catch (err) {
    // capture the not found error response and return null instead of throwing
    if (err?.response?.status === 404) {
      return null;
    }
    throw err;
  }
}

export function createBotToken(
  req: CreateBotJoinTokenRequest,
  mfaResponse?: MfaChallengeResponse
) {
  return api.post(
    cfg.getBotTokenUrl(),
    {
      integrationName: req.integrationName,
      joinMethod: req.joinMethod,
      webFlowLabel: req.webFlowLabel,
      gitHub: toApiGitHubTokenSpec(req.gitHub),
    },
    undefined /* abort signal */,
    mfaResponse
  );
}

export async function listBotTokens(
  variables: { botName: string; skipAuthnRetry: boolean },
  signal: AbortSignal
) {
  const path = cfg.getJoinTokenUrl({ action: 'listV2' });
  const qs = new URLSearchParams();
  qs.set('bot_name', variables.botName);
  qs.set('role', 'bot');

  try {
    const data = await api.get(`${path}?${qs.toString()}`, signal, undefined, {
      skipAuthnRetry: variables.skipAuthnRetry,
    });

    if (!validateListJoinTokensResponse(data)) {
      throw new Error('failed to validate list join tokens response');
    }

    return data;
  } catch (err) {
    // TODO(nicholasmarais1158) DELETE IN v20.0.0
    withGenericUnsupportedError(err, '19.0.0');
  }
}

export async function fetchBots(signal: AbortSignal, flags: FeatureFlags) {
  if (!flags.listBots) {
    throw new Error('cannot fetch bots: bots.list permission required');
  }

  return api
    .get(cfg.getBotUrl({ action: 'list' }), signal)
    .then((json: BotResponse) => {
      const items = json?.items || [];
      return { bots: items.map(makeBot) };
    });
}

export async function fetchRoles(
  variables: { search: string; flags: FeatureFlags },
  signal?: AbortSignal
): Promise<{ startKey: string; items: RoleResource[] }> {
  if (!variables.flags.roles) {
    return { startKey: '', items: [] };
  }

  const resourceSvc = new ResourceService();
  return resourceSvc.fetchRoles(
    { limit: 50, search: variables.search },
    signal
  );
}

export async function editBot(
  flags: FeatureFlags,
  botName: string,
  req: EditBotRequest
) {
  if (!flags.editBots) {
    throw new Error('cannot edit bot: bots.edit permission required');
  }
  if (!flags.roles) {
    throw new Error('cannot edit bot: roles.list permission required');
  }

  // TODO(nicholasmarais1158) DELETE IN v20.0.0
  const useV1 = canUseV1Edit(req);
  const path = useV1
    ? cfg.getBotUrl({ action: 'update', botName })
    : cfg.getBotUrl({ action: 'update-v2', botName });

  try {
    const res = await api.put(path, req);
    return makeBot(res);
  } catch (err: unknown) {
    // TODO(nicholasmarais1158) DELETE IN v20.0.0
    withGenericUnsupportedError(err, '17.6.1');
  }
}

export function deleteBot(flags: FeatureFlags, name: string) {
  if (!flags.removeBots) {
    throw new Error('cannot delete bot: bots.remove permission required');
  }

  return api.delete(cfg.getBotUrl({ action: 'delete', botName: name }));
}

export async function listBotInstances(
  variables: {
    pageToken: string;
    pageSize: number;
    searchTerm?: string;
    sort?: string;
    botName?: string;
  },
  signal?: AbortSignal
) {
  const { pageToken, pageSize, searchTerm, sort, botName } = variables;

  const path = cfg.getBotInstanceUrl({ action: 'list' });
  const qs = new URLSearchParams();

  qs.set('page_size', pageSize.toFixed());
  qs.set('page_token', pageToken);
  if (searchTerm) {
    qs.set('search', searchTerm);
  }
  if (sort) {
    qs.set('sort', sort);
  }
  if (botName) {
    qs.set('bot_name', botName);
  }

  const data = await api.get(`${path}?${qs.toString()}`, signal);

  if (!validateListBotInstancesResponse(data)) {
    throw new Error('failed to validate list bot instances response');
  }

  return data;
}

export async function getBotInstance(
  variables: {
    botName: string;
    instanceId: string;
  },
  signal?: AbortSignal
) {
  const path = cfg.getBotInstanceUrl({ action: 'read', ...variables });

  const data = await api.get(path, signal);

  if (!validateGetBotInstanceResponse(data)) {
    throw new Error('failed to validate get bot instance response');
  }

  return data;
}
