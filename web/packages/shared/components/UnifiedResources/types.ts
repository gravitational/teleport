/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { ResourceLabel } from 'teleport/services/agents';

import { ResourceIconName } from 'design/ResourceIcon';
import { Icon } from 'design/Icon';

import { DbProtocol } from 'shared/services/databases';
import { NodeSubKind } from 'shared/services';

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
};

export interface UnifiedResourceDatabase {
  kind: 'db';
  name: string;
  description: string;
  type: string;
  protocol: DbProtocol;
  labels: ResourceLabel[];
}

export interface UnifiedResourceNode {
  kind: 'node';
  id: string;
  hostname: string;
  labels: ResourceLabel[];
  addr: string;
  tunnel: boolean;
  subKind: NodeSubKind;
}

export interface UnifiedResourceKube {
  kind: 'kube_cluster';
  name: string;
  labels: ResourceLabel[];
}

export type UnifiedResourceDesktop = {
  kind: 'windows_desktop';
  os: 'windows' | 'linux' | 'darwin';
  name: string;
  addr: string;
  labels: ResourceLabel[];
};

export type UnifiedResourceUserGroup = {
  kind: 'user_group';
  name: string;
  description: string;
  labels: ResourceLabel[];
};

export type UnifiedResourceUi = {
  ActionButton: React.ReactElement;
};

export type SharedUnifiedResource = {
  resource:
    | UnifiedResourceApp
    | UnifiedResourceDatabase
    | UnifiedResourceNode
    | UnifiedResourceKube
    | UnifiedResourceDesktop
    | UnifiedResourceUserGroup;
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
  // TODO(bl-nero): Remove this once filters are expressed as advanced search.
  kinds?: string[];
};
export interface UnifiedResourceViewItem {
  name: string;
  description?: string;
  addr?: string;
  type: string;
  labels: {
    name: string;
    value: string;
  }[];
  primaryIconName: ResourceIconName;
  SecondaryIcon: typeof Icon;
  ActionButton: React.ReactElement;
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

export type ResourceItemProps = {
  name: string;
  primaryIconName: ResourceIconName;
  SecondaryIcon: typeof Icon;
  type: string;
  addr?: string;
  description?: string;
  labels: ResourceLabel[];
  ActionButton: React.ReactElement;
  onLabelClick?: (label: ResourceLabel) => void;
  pinResource: () => void;
  selectResource: () => void;
  selected: boolean;
  pinned: boolean;
  pinningSupport: PinningSupport;
};

export type UnifiedResourcesPinning =
  | {
      kind: 'supported';
      /** `getClusterPinnedResources` has to be stable, it is used in `useEffect`. */
      getClusterPinnedResources(): Promise<string[]>;
      updateClusterPinnedResources(pinned: string[]): Promise<void>;
    }
  | {
      kind: 'not-supported';
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
  mappedResources: { item: UnifiedResourceViewItem; key: string }[];
};
