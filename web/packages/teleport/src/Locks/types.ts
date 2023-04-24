/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { LabelDescription } from 'design/DataTable/types';
import { Option } from 'shared/components/Select';

import { AgentLabel } from 'teleport/services/agents';

export type Lock = {
  name: string;
  message: string;
  expires: string;
  createdAt: string;
  createdBy: string;
  targets: {
    user?: string;
    role?: string;
    login?: string;
    node?: string;
    mfa_device?: string;
    windows_desktop?: string;
    access_request?: string;
    device?: string;
  };
};

export type LockForTable = {
  name: string;
  message: string;
  expires: string;
  createdAt: string;
  createdBy: string;
  targets: LabelDescription[];
};

/**
 * AllowedTargets is the type of resource that a lock can be applied to.
 */
export type AllowedTargets =
  | 'user'
  | 'role'
  | 'login'
  | 'node'
  | 'mfa_device'
  | 'windows_desktop'
  | 'access_request'
  | 'device';

/**
 * TargetValue is the value of the target resource that a lock is applied to.
 * For example, if a TargetResource is 'node', its corresponding TargetValue should be
 * the node's UUID. If a TargetResource is 'role', its corresponding TargetValue should be
 * its name.
 */
export type TargetValue = string;

export type TableData = {
  // targetValue is not displayed in the table, but is the value
  // that will be used when creating the lock target
  targetValue: TargetValue;

  labels?: AgentLabel[];

  // these values are shown in the UI (each key is a separate column)
  [key: string]: any;
};

export type LockTarget = Option<AllowedTargets>;

export type SelectedLockTarget = {
  resource: AllowedTargets;
  targetValue: TargetValue;
};

export type OnAdd = (name: string) => void;

export type TargetListProps = {
  data: TableData[];
  onAdd: OnAdd;
  selectedResource: AllowedTargets;
  selectedLockTargets: SelectedLockTarget[];
};

export type CreateLockData = {
  targets: { [K in AllowedTargets]?: TargetValue };
  message?: string;
  ttl?: string;
};
