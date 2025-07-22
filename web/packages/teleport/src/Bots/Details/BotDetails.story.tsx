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

import { Meta, StoryObj } from '@storybook/react-vite';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createMemoryHistory } from 'history';
import { MemoryRouter, Route, Router } from 'react-router';

import Box from 'design/Box';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import { listBotInstancesSuccess } from 'teleport/test/helpers/botInstances';
import {
  editBotSuccess,
  getBotError,
  getBotForever,
  getBotSuccess,
} from 'teleport/test/helpers/bots';
import { successGetRoles } from 'teleport/test/helpers/roles';

import { BotDetails } from './BotDetails';

const meta = {
  title: 'Teleport/Bots/Details',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

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

const listBotInstancesSuccessHandler = listBotInstancesSuccess({
  bot_instances: [
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'svr-lon-01-ab23cd',
      join_method_latest: 'github',
      os_latest: 'linux',
      version_latest: '4.4.16',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'win-123a',
      join_method_latest: 'tpm',
      os_latest: 'windows',
      version_latest: '3.1.0+ab12hd',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'mac-007',
      join_method_latest: 'kubernetes',
      os_latest: 'darwin',
      version_latest: '2.9.99',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'aws:g49dh27dhjm3',
      join_method_latest: 'ec2',
      os_latest: 'linux',
      version_latest: '1.3.2',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'ios-build-42',
      join_method_latest: 'kubernetes',
      os_latest: 'darwin',
      version_latest: '5.0.0-dev',
    },
  ],
  next_page_token: '',
});

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listBotInstancesSuccessHandler,
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

export const HappyWithNoTraitsOrRolesOrInstances: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotSuccess({
          name: 'ansible-worker',
          roles: [],
          traits: [],
          max_session_ttl: {
            seconds: 43200,
          },
        }),
        listBotInstancesSuccess({
          bot_instances: [],
          next_page_token: '',
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

export const HappyWithoutEditPermission: Story = {
  args: {
    hasBotsEdit: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listBotInstancesSuccessHandler,
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

export const HappyWithoutBotInstanceListPermission: Story = {
  args: {
    hasBotInstanceListPermission: false,
  },
  parameters: {
    msw: {
      handlers: [
        successHandler,
        listBotInstancesSuccessHandler,
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
  parameters: {
    msw: {
      handlers: [getBotForever()],
    },
  },
};

export const WithFetchFailure: Story = {
  parameters: {
    msw: {
      handlers: [getBotError(500, 'error message goes here')],
    },
  },
};

export const WithBotNotFound: Story = {
  parameters: {
    msw: {
      handlers: [getBotError(404, 'not found')],
    },
  },
};

export const WithNoBotReadPermission: Story = {
  args: {
    hasBotsRead: false,
  },
  parameters: {
    msw: {
      handlers: [getBotError(500, 'you have permission, congrats 🎉')],
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

function Wrapper(props?: {
  hasBotsRead?: boolean;
  hasBotsEdit?: boolean;
  hasBotInstanceListPermission?: boolean;
}) {
  const {
    hasBotsRead = true,
    hasBotsEdit = true,
    hasBotInstanceListPermission = true,
  } = props ?? {};

  const history = createMemoryHistory({
    initialEntries: ['/web/bot/ansible-worker'],
  });

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
    botInstances: {
      ...defaultAccess,
      list: hasBotInstanceListPermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <TeleportProviderBasic teleportCtx={ctx}>
          <Router history={history}>
            <Route path={cfg.routes.bot}>
              <Box height={800} overflow={'auto'}>
                <BotDetails />
              </Box>
            </Route>
          </Router>
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
