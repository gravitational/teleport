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

import { Platform } from 'design/platform';
import { ResourceIconName } from 'design/ResourceIcon';
import { Resource } from 'gen-proto-ts/teleport/userpreferences/v1/onboard_pb';

import { RdsEngineIdentifier } from 'teleport/services/integrations';
import type { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';
import { AuthType } from 'teleport/services/user';
import type {
  DiscoverDiscoveryConfigMethod,
  DiscoverEventResource,
} from 'teleport/services/userEvent';
import { DiscoverGuideId } from 'teleport/services/userPreferences/discoverPreference';

import { ResourceKind } from '../Shared/ResourceKind';

export enum DatabaseLocation {
  Aws,
  SelfHosted,
  Gcp,
  Azure,
  Microsoft,

  TODO,
}

/** DatabaseEngine represents the db "protocol". */
export enum DatabaseEngine {
  Postgres,
  AuroraPostgres,
  MySql,
  AuroraMysql,
  MongoDb,
  Redis,
  CockroachDb,
  SqlServer,
  Snowflake,
  Cassandra,
  ElasticSearch,
  DynamoDb,
  Redshift,

  Doc,
}

export enum ServerLocation {
  Aws,
}

export enum KubeLocation {
  SelfHosted,
  Aws,
}

export interface ResourceSpec {
  /**
   * true if user pinned this guide
   */
  pinned?: boolean;
  dbMeta?: { location: DatabaseLocation; engine: DatabaseEngine };
  appMeta?: { awsConsole?: boolean };
  nodeMeta?: {
    location: ServerLocation;
    discoveryConfigMethod: DiscoverDiscoveryConfigMethod;
  };
  kubeMeta?: { location: KubeLocation };
  samlMeta?: { preset: SamlServiceProviderPreset };
  name: string;
  popular?: boolean;
  kind: ResourceKind;
  icon: ResourceIconName;
  /**
   * keywords are filter words that user may use to search for
   * this resource.
   */
  keywords: string[];
  /**
   * hasAccess is a flag to mean that user has
   * the preliminary permissions to add this resource.
   */
  hasAccess?: boolean;
  /**
   * unguidedLink is the link out to this resources documentation.
   * It is used as a flag, that when defined, means that
   * this resource is not "guided" (has no UI interactive flow).
   */
  unguidedLink?: string;
  /**
   * isDialog indicates whether the flow for this resource is a popover dialog as opposed to a
   * Discover flow. This is the case for the 'Application' resource.
   */
  isDialog?: boolean;
  /**
   * event is the expected backend enum event name that describes
   * the type of this resource (e.g. server v. kubernetes),
   * used for usage reporting.
   */
  event: DiscoverEventResource;
  /**
   * platform indicates a particular platform the resource is associated with.
   * Set this value if the resource should be prioritized based on the platform.
   */
  platform?: Platform;
  /**
   * supportedPlatforms indicate particular platforms the resource is available on. The resource
   * won't be displayed on unsupported platforms.
   *
   * An empty array or undefined means that the resource is supported on all platforms.
   */
  supportedPlatforms?: Platform[];
  /**
   * supportedAuthTypes indicate particular auth types that the resource is available for. The
   * resource won't be displayed if the user logged in using an unsupported auth type.
   *
   * An empty array or undefined means that the resource supports all auth types.
   */
  supportedAuthTypes?: AuthType[];
  id: DiscoverGuideId;
}

export enum SearchResource {
  UNSPECIFIED = '',
  APPLICATION = 'application',
  DATABASE = 'database',
  DESKTOP = 'desktop',
  KUBERNETES = 'kubernetes',
  SERVER = 'server',
  UNIFIED_RESOURCE = 'unified_resource',
}

export type PrioritizedResources = {
  preferredResources: Resource[];
  hasPreferredResources: boolean;
};

export function getRdsEngineIdentifier(
  engine: DatabaseEngine
): RdsEngineIdentifier {
  switch (engine) {
    case DatabaseEngine.MySql:
      return 'mysql';
    case DatabaseEngine.Postgres:
      return 'postgres';
    case DatabaseEngine.AuroraMysql:
      return 'aurora-mysql';
    case DatabaseEngine.AuroraPostgres:
      return 'aurora-postgres';
  }
}
