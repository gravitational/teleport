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

import { App } from 'teleport/services/apps';
import { Database } from 'teleport/services/databases';
import { Node } from 'teleport/services/nodes';
import { Kube } from 'teleport/services/kube';
import { Desktop } from 'teleport/services/desktops';

export type AgentKind = App | Database | Node | Kube | Desktop;

export type AgentResponse<T extends AgentKind> = {
  agents: T[];
  startKey?: string;
  totalCount?: number;
};

export type AgentLabel = {
  name: string;
  value: string;
};

export type AgentFilter = {
  // query is query expression using the predicate language.
  query?: string;
  // search contains search words/phrases separated by space.
  search?: string;
  sort?: SortType;
  limit?: number;
  startKey?: string;
};

export type SortType = {
  fieldName: string;
  dir: SortDir;
};

export type SortDir = 'ASC' | 'DESC';

// AgentIdKind are the same id constants used to mark the type of
// resource in the backend.
//
// These consts are expected for search based access requests.
export type AgentIdKind =
  | 'node'
  | 'app'
  | 'db'
  | 'kube_cluster'
  | 'windows_desktop';
