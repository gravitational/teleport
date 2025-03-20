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

import { useState } from 'react';

import type { TraitsOption } from 'shared/components/TraitsEditor';

import { AllUserTraits } from 'teleport/services/user';

import { UserAddEdit } from './UserAddEdit';

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
