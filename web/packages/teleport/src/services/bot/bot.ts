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

import api from 'teleport/services/api';
import cfg from 'teleport/config';

import {
  makeBot,
  makeListBot,
  toApiGitHubTokenSpec,
} from 'teleport/services/bot/consts';

import {
  BotList,
  BotResponse,
  FlatBot,
  CreateBotRequest,
  CreateBotJoinTokenRequest,
} from './types';

export function createBot(config: CreateBotRequest): Promise<void> {
  return api.post(cfg.getBotsUrl(), config);
}

export async function getBot(name: string): Promise<FlatBot | null> {
  try {
    return await api.get(cfg.getBotUrlWithName(name)).then(makeBot);
  } catch (err) {
    // capture the not found error response and return null instead of throwing
    if (err?.response?.status === 404) {
      return null;
    }
    throw err;
  }
}

export function createBotToken(req: CreateBotJoinTokenRequest) {
  return api.post(cfg.getBotTokenUrl(), {
    integrationName: req.integrationName,
    joinMethod: req.joinMethod,
    webFlowLabel: req.webFlowLabel,
    gitHub: toApiGitHubTokenSpec(req.gitHub),
  });
}

export function fetchBots(signal: AbortSignal): Promise<BotList> {
  return api.get(cfg.getBotsUrl(), signal).then((json: BotResponse) => {
    const items = json?.items || [];
    return { bots: items.map(makeListBot) };
  });
}

export function deleteBot(name: string) {
  return api.delete(cfg.getBotUrlWithName(name));
}
