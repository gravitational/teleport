/**
 * Copyright 2020 Gravitational, Inc.
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

import React, { useState } from 'react';

import { AllUserTraits } from 'teleport/services/user';

import { UserAddEdit } from './UserAddEdit';

import type { TraitsOption } from './TraitsEditor';

export default {
  title: 'Teleport/Users/UserAddEdit',
};

export const Create = () => {
  const p = {
    ...props,
    isNew: true,
    name: '',
    fetchRoles: async () => [],
    selectedRoles: [],
    attempt: { status: '' as const },
  };

  return <UserAddEdit {...p} />;
};

export const Edit = () => {
  const [configuredTraits, setConfiguredTraits] = useState([]);
  return (
    <UserAddEdit
      {...props}
      attempt={{ status: '' }}
      configuredTraits={configuredTraits}
      setConfiguredTraits={setConfiguredTraits}
    />
  );
  return <UserAddEdit {...props} attempt={{ status: '' }} />;
};

export const Processing = () => {
  return <UserAddEdit {...props} attempt={{ status: 'processing' }} />;
};

export const Failed = () => {
  return (
    <UserAddEdit
      {...props}
      attempt={{ status: 'failed', statusText: 'server error' }}
    />
  );
};

const props = {
  fetchRoles: async (input: string) =>
    ['Relupba', 'B', 'Pilhibo'].filter(r => r.includes(input)),
  onClose: () => null,
  selectedRoles: [
    { value: 'admin', label: 'admin' },
    { value: 'testrole', label: 'testrole' },
  ],
  name: 'lester',
  isNew: false,
  onChangeName() {},
  onChangeRoles() {},
  onSave() {},
  token: {
    value: '0c536179038b386728dfee6602ca297f',
    expires: new Date('2050-12-20T17:28:20.93Z'),
    username: 'Lester',
  },
  allTraits: { ['logins']: ['root', 'ubuntu'] } as AllUserTraits,
  configuredTraits: [
    {
      traitKey: { value: 'logins', label: 'logins' },
      traitValues: [{ value: 'root', label: 'root' }],
    },
  ] as TraitsOption[],
  setConfiguredTraits: () => null,
};
