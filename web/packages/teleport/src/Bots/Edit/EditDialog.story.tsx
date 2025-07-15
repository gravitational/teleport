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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  editBotError,
  editBotForever,
  editBotSuccess,
  getBotError,
  getBotForever,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';

import { EditDialog } from './EditDialog';

const meta = {
  title: 'Teleport/Bots/Edit',
  component: Wrapper,
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

const beforeEach = () => {
  queryClient.clear(); // Prevent cached data sharing between stories
};

const successHandler = getBotSuccess({
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
});

export const Happy: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [
        successHandler,
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

export const WithFetchPending: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [getBotForever()],
    },
  },
};

export const WithFetchFailure: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [getBotError(500, 'error message goes here')],
    },
  },
};

export const WithBotNotFound: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [getBotError(404, 'not found')],
    },
  },
};

export const WithNoBotReadPermission: Story = {
  beforeEach,
  args: {
    hasBotsRead: false,
  },
  parameters: {
    msw: {
      handlers: [getBotError(500, 'you have permission, congrats 🎉')],
    },
  },
};

export const WithNoBotEditPermission: Story = {
  beforeEach,
  args: {
    hasBotsEdit: false,
  },
  parameters: {
    msw: {
      handlers: [successHandler],
    },
  },
};

export const WithSubmitPending: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [
        successHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotForever(),
      ],
    },
  },
};

export const WithSubmitFailure: Story = {
  beforeEach,
  parameters: {
    msw: {
      handlers: [
        successHandler,
        successGetRoles({
          startKey: '',
          items: ['access', 'editor', 'terraform-provider'].map(r => ({
            content: r,
            id: r,
            name: r,
            kind: 'role',
          })),
        }),
        editBotError(500, 'something went wrong'),
      ],
    },
  },
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});

function Wrapper(props?: { hasBotsRead?: boolean; hasBotsEdit?: boolean }) {
  const { hasBotsRead = true, hasBotsEdit = true } = props ?? {};

  const customAcl = makeAcl({
    bots: {
      ...defaultAccess,
      read: hasBotsRead,
      edit: hasBotsEdit,
    },
    roles: {
      ...defaultAccess,
      list: true,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <TeleportProviderBasic teleportCtx={ctx}>
          <EditDialog
            botName="ansible-worker"
            onCancel={() => {}}
            onSuccess={() => {}}
          />
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
