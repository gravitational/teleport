/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { NavItem } from 'teleterm/ui/Navigator/types';

export interface ClusterNavItem extends NavItem {
  connected: boolean;
  syncing: boolean;
  leaves?: ClusterNavItem[];
  clusterUri: string;
  active: boolean;
}

export interface ExpanderClusterState {
  items: ClusterNavItem[];
  onAddCluster?(): void;
  onSyncClusters?(): void;
  onOpenContextMenu?(item: ClusterNavItem): void;
  onOpen(clusterUri: string): void;
}
