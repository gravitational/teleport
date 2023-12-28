import api from 'teleport/services/api';
import cfg from 'teleport/config';

import type { BotConfig } from './types';

export const botService = {
  createBot(config: BotConfig): Promise<void> {
    return api.post(cfg.getBotUrl(cfg.proxyCluster), config);
  },
};
