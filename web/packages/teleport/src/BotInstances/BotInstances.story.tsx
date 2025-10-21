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
import { MemoryRouter, Router } from 'react-router';

import Box from 'design/Box/Box';

import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  getBotInstanceError,
  getBotInstanceMetricsSuccess,
  getBotInstanceSuccess,
  listBotInstancesError,
  listBotInstancesForever,
  listBotInstancesSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstances } from './BotInstances';

const meta = {
  title: 'Teleport/BotInstances',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

const listBotInstances = {
  bot_instances: [
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'my-svc.my-namespace.svc.cluster-domain.example',
      join_method_latest: 'github',
      os_latest: 'linux',
      version_latest: '4.4.0',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'win-123a',
      join_method_latest: 'tpm',
      os_latest: 'windows',
      version_latest: '4.3.18+ab12hd',
    },
    {
      bot_name: 'ansible-worker',
      instance_id: crypto.randomUUID(),
      active_at_latest: '2025-07-22T10:54:00Z',
      host_name_latest: 'mac-007',
      join_method_latest: 'kubernetes',
      os_latest: 'darwin',
      version_latest: '3.9.99',
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
    },
  ],
  next_page_token: '',
};

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [
        listBotInstancesSuccess(listBotInstances, 'v1'),
        listBotInstancesSuccess(listBotInstances, 'v2'),
        getBotInstanceSuccess(),
        getBotInstanceMetricsSuccess(),
      ],
    },
  },
};

export const ErrorLoadingList: Story = {
  parameters: {
    msw: {
      handlers: [
        listBotInstancesError(500, 'something went wrong'),
        getBotInstanceMetricsSuccess(),
      ],
    },
  },
};

export const StillLoadingList: Story = {
  parameters: {
    msw: {
      handlers: [listBotInstancesForever(), getBotInstanceMetricsSuccess()],
    },
  },
};

export const NoListPermission: Story = {
  args: {
    hasBotInstanceListPermission: false,
  },
  parameters: {
    msw: {
      handlers: [
        listBotInstancesError(
          500,
          'this call should never be made without permissions'
        ),
        getBotInstanceMetricsSuccess(),
      ],
    },
  },
};

export const NoReadPermission: Story = {
  args: {
    hasBotInstanceReadPermission: false,
  },
  parameters: {
    msw: {
      handlers: [
        listBotInstancesSuccess(listBotInstances, 'v1'),
        listBotInstancesSuccess(listBotInstances, 'v2'),
        getBotInstanceError(
          500,
          'this call should never be made without permissions'
        ),
        getBotInstanceMetricsSuccess(),
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

function Wrapper(props?: {
  hasBotInstanceListPermission?: boolean;
  hasBotInstanceReadPermission?: boolean;
}) {
  const {
    hasBotInstanceListPermission = true,
    hasBotInstanceReadPermission = true,
  } = props ?? {};

  const history = createMemoryHistory({
    initialEntries: ['/web/bots/instances'],
  });

  const customAcl = makeAcl({
    botInstances: {
      ...defaultAccess,
      read: hasBotInstanceReadPermission,
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
            <Route path={cfg.routes.botInstances}>
              <Box height={820}>
                <BotInstances />
              </Box>
            </Route>
          </Router>
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
