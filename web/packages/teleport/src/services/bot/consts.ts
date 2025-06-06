/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import {
  ApiBot,
  BotType,
  BotUiFlow,
  FlatBot,
  GetBotInstanceResponse,
  GitHubRepoRule,
  ListBotInstancesResponse,
  ProvisionTokenSpecV2GitHub,
} from 'teleport/services/bot/types';

/**
 *
 * @param spec a ProvisionTokenSpecV2GitHub
 * @returns the server's teleport/api/types.ProvisionTokenSpecV2GitHub,
 * which has similar properties but different casing
 */
export function toApiGitHubTokenSpec(spec: ProvisionTokenSpecV2GitHub | null) {
  if (!spec) {
    return null;
  }
  return {
    allow: spec.allow.map(toApiGitHubRule),
    enterprise_server_host: spec.enterpriseServerHost,
  };
}

/**
 *
 * @param {GitHubRepoRule} rule a GitHubRepoRule
 * @returns the server's teleport/api/types.ProvisionTokenSpecV2GitHub_Rule,
 * which has similar properties, but different casing
 */
export function toApiGitHubRule({
  sub,
  repository,
  repositoryOwner,
  workflow,
  environment,
  actor,
  ref,
  refType,
}: GitHubRepoRule) {
  return {
    sub,
    repository,
    repository_owner: repositoryOwner,
    workflow,
    environment,
    actor,
    ref,
    ref_type: refType,
  };
}

export function makeBot(bot: ApiBot): FlatBot {
  if (!bot?.metadata?.name) {
    return;
  }

  const labels = bot?.metadata?.labels
    ? new Map(Object.entries(bot.metadata.labels))
    : new Map<string, string>();

  return {
    kind: bot?.kind || '',
    status: bot?.status || '',
    subKind: bot?.subKind || '',
    version: bot?.version || '',

    name: bot?.metadata?.name || '',
    namespace: bot?.metadata?.namespace || '',
    description: bot?.metadata?.description || '',
    labels: labels,
    revision: bot?.metadata?.revision || '',
    type: getBotType(labels),

    roles: bot?.spec?.roles || [],
    traits: bot?.spec?.traits || [],
  };
}

export function parseListBotInstancesResponse(
  data: unknown
): data is ListBotInstancesResponse {
  if (typeof data !== 'object' || data === null) {
    return false;
  }

  if (!('bot_instances' in data)) {
    return false;
  }

  if (!Array.isArray(data.bot_instances)) {
    return false;
  }

  return data.bot_instances.every(x => typeof x === 'object' || x !== null);
}

export function parseGetBotInstanceResponse(
  data: unknown
): data is GetBotInstanceResponse {
  if (typeof data !== 'object' || data === null) {
    return false;
  }

  if (!('bot_instance' in data && 'yaml' in data)) {
    return false;
  }

  if (typeof data.bot_instance !== 'object' || data.bot_instance === null) {
    return false;
  }

  return true;
}

export function getBotType(labels: Map<string, string>): BotType {
  if (!labels) {
    return null;
  }

  for (let [key, value] of labels) {
    if (key === GITHUB_ACTIONS_LABEL_KEY) {
      if (Object.values(BotUiFlow).includes(value as BotUiFlow)) {
        return value as BotUiFlow;
      }
    }
  }

  return null;
}

export const GITHUB_ACTIONS_LABEL_KEY = 'teleport.internal/ui-flow';
