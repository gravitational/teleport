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

import { ResourceLabel } from '../agents';
import { JoinMethod } from '../joinToken';

export type CreateBotRequest = {
  botName: string;
  labels: ResourceLabel[];
  roles: string[];
  login: string;
};

export type ApiBotMetadata = {
  description: string;
  labels: Map<string, string>;
  name: string;
  namespace: string;
  revision: string;
};

export type ApiBotSpec = {
  roles: string[];
  traits: ApiBotTrait[];
};

export type ApiBotTrait = {
  name: string;
  values: string[];
};

export type ApiBot = {
  kind: string;
  metadata: ApiBotMetadata;
  spec: ApiBotSpec;
  status: string;
  subKind: string;
  version: string;
};

export type ListBotInstancesResponse = {
  bot_instances: BotInstanceSummary[];
  next_page_token?: string;
};

export type BotInstanceSummary = {
  instance_id: string;
  bot_name: string;
  join_method_latest?: string;
  host_name_latest?: string;
  version_latest?: string;
  active_at_latest?: string;
};

export type GetBotInstanceResponse = {
  bot_instance?: {
    spec?: {
      instance_id?: string;
    } | null;
  } | null;
  yaml?: string;
};

export type BotList = {
  bots: FlatBot[];
};

export type FlatBot = Omit<ApiBot, 'metadata' | 'spec'> &
  ApiBotMetadata &
  ApiBotSpec & { type?: BotType };

export type BotResponse = {
  items: ApiBot[];
};

export type CreateBotJoinTokenRequest = {
  integrationName: string;
  joinMethod: JoinMethod;
  gitHub?: ProvisionTokenSpecV2GitHub;
  webFlowLabel: string;
};

export type ProvisionTokenSpecV2GitHub = {
  allow: GitHubRepoRule[];
  enterpriseServerHost: string;
};

export type RefType = 'branch' | 'tag';

export type GitHubRepoRule = {
  sub?: string;
  repository: string;
  repositoryOwner: string;
  workflow?: string;
  environment?: string;
  actor?: string;
  ref?: string;
  refType?: RefType;
};

export type BotType = BotUiFlow;

export enum BotUiFlow {
  GitHubActionsSsh = 'github-actions-ssh',
}
export type EditBotRequest = {
  // roles is the list of roles to assign to the bot
  roles: string[];
};
