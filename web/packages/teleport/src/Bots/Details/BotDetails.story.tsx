/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { Meta, StoryObj } from '@storybook/react';
import { MemoryRouter } from 'react-router';

import { TeleportProviderBasic } from 'teleport/mocks/providers';
import {
  editBotSuccess,
  getBotError,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';

import { BotDetails } from './BotDetails';

const meta = {
  title: 'Teleport/Bots/Details',
  component: Details,
} satisfies Meta<typeof Details>;

type Story = StoryObj<typeof meta>;

export default meta;

export const DetailsWithFetchFailure: Story = {
  parameters: {
    msw: {
      handlers: [getBotError(500, 'error message goes here')],
    },
  },
};

export const DetailsWithFetchSuccess: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotSuccess({
          name: 'ansible-worker',
          roles: ['terraform-provider'],
          traits: [
            {
              name: 'logins',
              values: ['guest'],
            },
            {
              name: 'db_names',
              values: ['*'],
            },
            {
              name: 'custom_idp',
              values: ['val-1', 'val-2', 'val-3'],
            },
          ],
          max_session_ttl: {
            seconds: 43200,
          },
        }),
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotSuccess(),
      ],
    },
  },
};

function Details() {
  return (
    <MemoryRouter>
      <TeleportProviderBasic>
        <BotDetails />
      </TeleportProviderBasic>
    </MemoryRouter>
  );
}
