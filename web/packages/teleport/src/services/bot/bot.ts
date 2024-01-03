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

import type { Bot, BotConfig } from './types';

export const botService = {
  createBot(config: BotConfig): Promise<void> {
    return api.post(cfg.getBotUrl(cfg.proxyCluster), config);
  },

  async getBot(name: string): Promise<Bot | null> {
    try {
      return await api.get(cfg.getBotUrl(cfg.proxyCluster, name)).then(makeBot);
    } catch (err) {
      // capture the not found error response and return null instead of throwing
      if (err?.response?.status === 404) {
        return null;
      }
      throw err;
    }
  },
};

function makeBot(json): Bot {
  return {
    name: json.metadata.name,
    roles: json.spec.roles,
    traits: json.spec.traits,
  };
}
