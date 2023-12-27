import api from 'teleport/services/api';
import cfg from 'teleport/config';

import type { GitHubBotConfig } from './types'

export const botService = {
  createGitHubBot(config: GitHubBotConfig): Promise<void> {
    return api.
      post(cfg.getBotUrl(cfg.proxyCluster, "github-actions"), config)
  }
}