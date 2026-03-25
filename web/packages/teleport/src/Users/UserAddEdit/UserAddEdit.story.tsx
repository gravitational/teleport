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

import type { StoryObj } from '@storybook/react-vite';
import { delay } from 'msw';

import type { TraitsOption } from 'shared/components/TraitsEditor';

import { TeleportProviderBasic } from 'teleport/mocks/providers';
import type { RoleResource } from 'teleport/services/resources';
import { AllUserTraits } from 'teleport/services/user';
import { successGetRoles } from 'teleport/test/helpers/roles';
import { handleUpdateUser } from 'teleport/test/helpers/users';

import { UserAddEdit } from './UserAddEdit';

export default {
  title: 'Teleport/Users/UserAddEdit',
};

export const Create: StoryObj = {
  render() {
    const p = {
      ...props,
      isNew: true,
      name: '',
      fetchRoles: async () => [],
      modifyFetchedData: () => null,
      selectedRoles: [],
      user: {
        name: '',
        roles: [],
        authType: 'local',
        isLocal: true,
      },
    };

    return (
      <TeleportProviderBasic>
        <UserAddEdit {...p} />
      </TeleportProviderBasic>
    );
  },
};

const roles: RoleResource[] = [
  { id: '1', name: 'Relupba', content: '', kind: 'role' },
  { id: '2', name: 'B', content: '', kind: 'role' },
  { id: '3', name: 'Pilhibo', content: '', kind: 'role' },
];

export const Edit: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        successGetRoles({
          startKey: '',
          items: roles,
        }),
      ],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserAddEdit {...props} />
      </TeleportProviderBasic>
    );
  },
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleUpdateUser(() => delay('infinite'))],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserAddEdit {...props} />
      </TeleportProviderBasic>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleUpdateUser(() => delay('infinite'))],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserAddEdit {...props} />
      </TeleportProviderBasic>
    );
  },
};

const props = {
  fetchRoles: async (input: string) =>
    ['Relupba', 'B', 'Pilhibo'].filter(r => r.includes(input)),
  onClose: () => null,
  selectedRoles: [
    { value: 'admin', label: 'admin' },
    { value: 'testrole', label: 'testrole' },
  ],
  user: {
    name: 'lester',
    roles: ['editor'],
  },
  modifyFetchedData: () => null,
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
