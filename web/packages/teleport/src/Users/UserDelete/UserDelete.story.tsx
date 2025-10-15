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

import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { errorDeleteUser, handleDeleteUser } from 'teleport/test/helpers/users';

import { UserDelete } from './UserDelete';

export default {
  title: 'Teleport/Users/UserDelete',
};

export const Processing: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleDeleteUser(() => delay('infinite'))],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserDelete {...props} />
      </TeleportProviderBasic>
    );
  },
};

export const Confirm: StoryObj = {
  parameters: {
    msw: {
      handlers: [handleDeleteUser(() => delay('infinite'))],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserDelete {...props} />
      </TeleportProviderBasic>
    );
  },
};

export const Failed: StoryObj = {
  parameters: {
    msw: {
      handlers: [errorDeleteUser('server error')],
    },
  },
  render() {
    return (
      <TeleportProviderBasic>
        <UserDelete {...props} />
      </TeleportProviderBasic>
    );
  },
};

const props = {
  username: 'somename',
  onDelete: () => null,
  onClose: () => null,
  modifyFetchedData: () => null,
};
