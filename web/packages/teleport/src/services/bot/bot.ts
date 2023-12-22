import api from 'teleport/services/api';
import cfg from 'teleport/config';

import type { Bot, GitHubBotConfig } from './types'

export const botService = {
  createGitHubBot(config: GitHubBotConfig): Promise<void> {
    return api.
      post(cfg.getBotUrl(cfg.proxyCluster, "github"), {
        botName: config.name,
        botRoles: config.roles,
        repository: config.repository,
        subject: config.subject,
        repositoryOwner: config.repositoryOwner,
        workflow: config.workflow,
        environment: config.environment,
        actor: config.actor,
        ref: config.ref,
        refType: config.refType,
      })
  }
}