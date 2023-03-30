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

export type AllowedTargets =
  | 'user'
  | 'role'
  | 'login'
  | 'node'
  | 'mfa_device'
  | 'windows_desktop'
  | 'access_request'
  | 'device';

export type TableData = {
  [key: string]: string;
};

export type LockTarget = {
  label: string;
  value: AllowedTargets;
};

export type SelectedLockTarget = {
  type: AllowedTargets;
  name: string;
};

export type OnAdd = (name: string) => void;

export type TargetListProps = {
  data: TableData[];
  onAdd: OnAdd;
  selectedTarget: AllowedTargets;
  selectedLockTargets: SelectedLockTarget[];
};

export type CreateLockData = {
  targets: { [K in AllowedTargets]?: string };
  message?: string;
  ttl?: string;
};
