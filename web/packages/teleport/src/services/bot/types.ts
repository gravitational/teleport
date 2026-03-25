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
  max_session_ttl:
    | {
        seconds: number;
      }
    | undefined;
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
  os_latest?: string;
};

export type GetBotInstanceResponse = {
  bot_instance?: {
    spec?: {
      instance_id?: string;
      bot_name?: string;
    } | null;
    status?: {
      latest_heartbeats?:
        | {
            uptime?: {
              seconds?: number;
            } | null;
            version?: string;
            os?: string;
            hostname?: string;
            kind?: BotInstanceKind;
          }[]
        | null;
      latest_authentications?:
        | {
            join_attrs?: GetBotInstanceResponseJoinAttrs | null;
          }[]
        | null;
      service_health?:
        | {
            service?: {
              type?: string;
              name?: string;
            } | null;
            status?: BotInstanceServiceHealthStatus;
            reason?: string;
            updated_at?: { seconds: number } | null;
          }[]
        | null;
    };
  } | null;
  yaml?: string;
};

export enum BotInstanceServiceHealthStatus {
  BOT_INSTANCE_HEALTH_STATUS_UNSPECIFIED = 0,
  BOT_INSTANCE_HEALTH_STATUS_INITIALIZING = 1,
  BOT_INSTANCE_HEALTH_STATUS_HEALTHY = 2,
  BOT_INSTANCE_HEALTH_STATUS_UNHEALTHY = 3,
}

export enum BotInstanceKind {
  BOT_KIND_UNSPECIFIED = 0,
  BOT_KIND_TBOT = 1,
  BOT_KIND_TERRAFORM_PROVIDER = 2,
  BOT_KIND_KUBERNETES_OPERATOR = 3,
  BOT_KIND_TCTL = 4,
}

export type GetBotInstanceResponseJoinAttrs = {
  meta?: {
    join_token_name?: string;
    join_method?: string;
  } | null;
  gitlab?: {
    sub?: string;
    project_path?: string;
  } | null;
  github?: {
    sub?: string;
    repository?: string;
  } | null;
  iam?: {
    account?: string;
    arn?: string;
  } | null;
  tpm?: {
    ek_pub_hash?: string;
  } | null;
  azure?: {
    subscription?: string;
    resource_group?: string;
  } | null;
  circleci?: {
    sub?: string;
    project_id?: string;
  } | null;
  bitbucket?: {
    sub?: string;
    repository_uuid?: string;
  } | null;
  terraform_cloud?: {
    sub?: string;
    full_workspace?: string;
  } | null;
  spacelift?: {
    sub?: string;
    space_id?: string;
  } | null;
  gcp?: {
    service_account?: string;
  } | null;
  kubernetes?: {
    subject?: string;
  } | null;
  oracle?: {
    tenancy_id?: string;
    compartment_id?: string;
  } | null;
  azure_devops?: {
    pipeline?: {
      sub?: string;
      repository_id?: string;
    } | null;
  } | null;
};

export type GetBotInstanceMetricsResponse = {
  upgrade_statuses?: {
    unsupported?: BotInstanceMetric | null;
    patch_available?: BotInstanceMetric | null;
    requires_upgrade?: BotInstanceMetric | null;
    up_to_date?: BotInstanceMetric | null;
    updated_at?: string;
  } | null;
  refresh_after_seconds: number;
};

type BotInstanceMetric = {
  count?: number;
  filter?: string;
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
  roles?: string[] | null;
  // traits is the list of traits to assign to the bot
  traits?: ApiBotTrait[] | null;
  // max_session_ttl is the maximum session TTL
  max_session_ttl?: string | null;
  // description is the bot's metadata description
  description?: string | null;
};
