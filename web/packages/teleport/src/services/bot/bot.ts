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

import { makeBot } from 'teleport/services/bot/consts';
import { BotsResponse, FlatBot } from 'teleport/Bots/types';

import type { FetchBotsRequest, CreateBotRequest } from './types';

export function fetchBots({ signal }: FetchBotsRequest): Promise<BotsResponse> {
  return api
    .get(cfg.getBotsUrl(cfg.proxyCluster), signal)
    .then(json => {
      const items = json?.items || [];
      return {
        bots: items.map(makeBot),
        startKey: json?.startKey,
        totalCount: json?.totalCount,
      };
    })
    .catch(res => {
      throw res;
    });
}

// TODO: remove service, leave methods hanging from file
export const botService = {
  createBot(config: CreateBotRequest): Promise<void> {
    return api.post(cfg.getBotsUrl(cfg.proxyCluster), config);
  },

  async getBot(name: string): Promise<FlatBot | null> {
    try {
      return await api
        .get(cfg.getBotsUrl(cfg.proxyCluster, name))
        .then(makeBot);
    } catch (err) {
      // capture the not found error response and return null instead of throwing
      if (err?.response?.status === 404) {
        return null;
      }
      throw err;
    }
  },
};
