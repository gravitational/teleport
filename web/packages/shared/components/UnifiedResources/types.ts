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

import React from 'react';

import { Icon } from 'design/Icon';
import { ResourceIconName } from 'design/ResourceIcon';
import { AppSubKind, NodeSubKind } from 'shared/services';
import { DbProtocol } from 'shared/services/databases';

// eslint-disable-next-line no-restricted-imports -- FIXME
import { ResourceLabel } from 'teleport/services/agents';
// eslint-disable-next-line no-restricted-imports -- FIXME
import { AppMCP, PermissionSet } from 'teleport/services/apps';

// "mixed" indicates the resource has a mix of health
// statuses. This can happen when multiple agents proxy the same resource.
export type ResourceHealthStatus =
  | 'healthy'
  | 'unhealthy'
  | 'unknown'
  | 'mixed';

const resourceHealthStatuses = new Set<ResourceHealthStatus>([
  'healthy',
  'unhealthy',
  'unknown',
  'mixed',
]);

export function isResourceHealthStatus(
  status: unknown
): status is ResourceHealthStatus {
  return (
    typeof status === 'string' &&
    resourceHealthStatuses.has(status as ResourceHealthStatus)
  );
}

export type ResourceTargetHealth = {
  status: ResourceHealthStatus;
  // additional information meant for user.
  message?: string;
  error?: string;
};

export type UnifiedResourceApp = {
  kind: 'app';
  id: string;
  name: string;
  description: string;
  labels: ResourceLabel[];
  awsConsole: boolean;
  addrWithProtocol?: string;
  friendlyName?: string;
  samlApp: boolean;
  requiresRequest?: boolean;
  subKind?: AppSubKind;
  permissionSets?: PermissionSet[];
  mcp?: AppMCP;
};

export interface UnifiedResourceDatabase {
  kind: 'db';
  name: string;
  description: string;
  type: string;
  protocol: DbProtocol;
  labels: ResourceLabel[];
  requiresRequest?: boolean;
  targetHealth?: ResourceTargetHealth;
}

export interface UnifiedResourceNode {
  kind: 'node';
  id: string;
  hostname: string;
  labels: ResourceLabel[];
  addr: string;
  tunnel: boolean;
  subKind: NodeSubKind;
  requiresRequest?: boolean;
}

export interface UnifiedResourceKube {
  kind: 'kube_cluster';
  name: string;
  labels: ResourceLabel[];
  requiresRequest?: boolean;
}

export type UnifiedResourceDesktop = {
  kind: 'windows_desktop';
  os: 'windows' | 'linux' | 'darwin';
  name: string;
  addr: string;
  labels: ResourceLabel[];
  requiresRequest?: boolean;
};

export type UnifiedResourceUserGroup = {
  kind: 'user_group';
  name: string;
  description: string;
  friendlyName?: string;
  labels: ResourceLabel[];
  requiresRequest?: boolean;
};

export interface UnifiedResourceGitServer {
  kind: 'git_server';
  id: string;
  hostname: string;
  labels: ResourceLabel[];
  subKind: 'github';
  github: {
    organization: string;
    integration: string;
  };
  requiresRequest?: boolean;
}

export type UnifiedResourceUi = {
  ActionButton: React.ReactElement;
};

export type UnifiedResourceDefinition =
  | UnifiedResourceApp
  | UnifiedResourceDatabase
  | UnifiedResourceNode
  | UnifiedResourceKube
  | UnifiedResourceDesktop
  | UnifiedResourceUserGroup
  | UnifiedResourceGitServer;

export type SharedUnifiedResource = {
  resource: UnifiedResourceDefinition;
  ui: UnifiedResourceUi;
};

export type UnifiedResourcesQueryParams = {
  /** query is query expression using the predicate language. */
  query?: string;
  /** search contains search words/phrases separated by space. */
  search?: string;
  sort?: {
    fieldName: string;
    dir: 'ASC' | 'DESC';
  };
  pinnedOnly?: boolean;
  statuses?: ResourceHealthStatus[];
  // TODO(bl-nero): Remove this once filters are expressed as advanced search.
  kinds?: string[];
  includedResourceMode?: IncludedResourceMode;
};
export interface UnifiedResourceViewItem {
  name: string;
  labels: {
    name: string;
    value: string;
  }[];
  primaryIconName: ResourceIconName;
  SecondaryIcon: typeof Icon;
  ActionButton: React.ReactElement;
  cardViewProps: CardViewSpecificProps;
  listViewProps: ListViewSpecificProps;
  requiresRequest?: boolean;
  status?: ResourceHealthStatus;
}

export enum PinningSupport {
  Supported = 'Supported',
  /**
   * Disables pinning functionality if a leaf cluster hasn't been upgraded yet.
   * Shows an appropriate message on hover.
   * */
  NotSupported = 'NotSupported',
  /** Disables the pinning button. */
  Disabled = 'Disabled',
  /** Hides the pinning button completely. */
  Hidden = 'Hidden',
}

export type IncludedResourceMode =
  | 'none'
  | 'all'
  | 'requestable'
  | 'accessible';

export type ResourceItemProps = {
  onLabelClick?: (label: ResourceLabel) => void;
  pinResource: () => void;
  selectResource: () => void;
  selected: boolean;
  pinned: boolean;
  pinningSupport: PinningSupport;
  expandAllLabels: boolean;
  viewItem: UnifiedResourceViewItem;
  onShowStatusInfo(): void;
  /**
   * True, when the InfoGuideSidePanel is opened.
   * Used as a flag to render different styling.
   */
  showingStatusInfo: boolean;
};

// Props that are needed for the Card view.
// The reason we need this separately defined is because unlike with the list view, what we display in the
// description sections of a card varies based on the type of its resource. For example, for applications,
// instead of showing the `Application` type under the name like we would for other resources, we show the description.
type CardViewSpecificProps = {
  primaryDesc?: string;
  secondaryDesc?: string;
};

type ListViewSpecificProps = {
  description?: string;
  addr?: string;
  resourceType: string;
};

export type UnifiedResourcesPinning =
  | {
      kind: 'supported';
      /** `getClusterPinnedResources` has to be stable, it is used in `useEffect`. */
      getClusterPinnedResources(): Promise<string[]>;
      updateClusterPinnedResources(pinned: string[]): Promise<void>;
    }
  | {
      kind: 'hidden';
    };

export type ResourceViewProps = {
  onLabelClick: (label: ResourceLabel) => void;
  pinnedResources: string[];
  selectedResources: string[];
  onSelectResource: (resourceId: string) => void;
  onPinResource: (resourceId: string) => void;
  pinningSupport: PinningSupport;
  isProcessing: boolean;
  mappedResources: {
    item: UnifiedResourceViewItem;
    key: string;
    onShowStatusInfo(): void;
    showingStatusInfo: boolean;
  }[];
  expandAllLabels: boolean;
};

/**
 * DatabaseServer (db_server) describes a database heartbeat signal
 * reported from an agent (db_service) that is proxying
 * the database.
 */
export type DatabaseServer = {
  kind: 'db_server';
  hostname: string;
  hostId: string;
  targetHealth?: ResourceTargetHealth;
};

export type SharedResourceServer = DatabaseServer;
