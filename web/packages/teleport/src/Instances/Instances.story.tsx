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
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter, Route, Router } from 'react-router';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import { Instances } from './Instances';

const meta = {
  title: 'Teleport/Instance Inventory',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear();
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

const regularInstances = [
  {
    id: crypto.randomUUID(),
    type: 'instance',
    instance: {
      name: 'ip-10-1-1-100.ec2.internal',
      version: '18.2.4',
      services: ['node', 'proxy'],
      upgrader: {
        type: 'systemd-unit-updater',
        group: 'production',
      },
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'instance',
    instance: {
      name: 'teleport-auth-01',
      version: '18.2.3',
      services: ['auth'],
      upgrader: {
        type: 'kube-updater',
        group: 'staging',
      },
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'instance',
    instance: {
      name: 'app-server-prod',
      version: '18.1.0',
      services: ['app', 'db'],
    },
  },
];

const botInstances = [
  {
    id: crypto.randomUUID(),
    type: 'bot_instance',
    botInstance: {
      name: 'github-actions-bot',
      version: '18.2.4',
    },
  },
  {
    id: crypto.randomUUID(),
    type: 'bot_instance',
    botInstance: {
      name: 'ci-cd-bot',
      version: '18.2.2',
    },
  },
];

const mockInstances = {
  instances: [...regularInstances, ...botInstances],
  startKey: '',
};

const mockOnlyRegularInstances = {
  instances: regularInstances,
  startKey: '',
};

const mockOnlyBotInstances = {
  instances: botInstances,
  startKey: '',
};

const listInstancesSuccess = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockInstances);
  }
);

const listOnlyRegularInstances = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockOnlyRegularInstances);
  }
);

const listOnlyBotInstances = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  () => {
    return HttpResponse.json(mockOnlyBotInstances);
  }
);

const listInstancesError = (status: number, message: string) =>
  http.get('/v1/webapi/sites/:clusterId/instances', () => {
    return HttpResponse.json({ error: { message } }, { status });
  });

const listInstancesLoading = http.get(
  '/v1/webapi/sites/:clusterId/instances',
  async () => {
    await delay('infinite');
    return HttpResponse.json(mockInstances);
  }
);

export const Loaded: Story = {
  parameters: {
    msw: {
      handlers: [listInstancesSuccess],
    },
  },
};

export const CacheInitializing: Story = {
  parameters: {
    msw: {
      handlers: [listInstancesError(503, 'inventory cache is not yet healthy')],
    },
  },
};

export const Loading: Story = {
  parameters: {
    msw: {
      handlers: [listInstancesLoading],
    },
  },
};

export const Error: Story = {
  parameters: {
    msw: {
      handlers: [listInstancesError(500, 'some error')],
    },
  },
};

export const NoInstancePermissions: Story = {
  args: {
    hasInstanceListPermission: false,
    hasInstanceReadPermission: false,
  },
  parameters: {
    msw: {
      handlers: [listOnlyBotInstances],
    },
  },
};

export const NoBotInstancePermissions: Story = {
  args: {
    hasBotInstanceListPermission: false,
    hasBotInstanceReadPermission: false,
  },
  parameters: {
    msw: {
      handlers: [listOnlyRegularInstances],
    },
  },
};

export const NoPermissionsAtAll: Story = {
  args: {
    hasInstanceListPermission: false,
    hasInstanceReadPermission: false,
    hasBotInstanceListPermission: false,
    hasBotInstanceReadPermission: false,
  },
  parameters: {
    msw: {
      handlers: [listInstancesError(403, 'access denied')],
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
  hasInstanceListPermission?: boolean;
  hasInstanceReadPermission?: boolean;
  hasBotInstanceListPermission?: boolean;
  hasBotInstanceReadPermission?: boolean;
}) {
  const {
    hasInstanceListPermission = true,
    hasInstanceReadPermission = true,
    hasBotInstanceListPermission = true,
    hasBotInstanceReadPermission = true,
  } = props ?? {};

  const history = createMemoryHistory({
    initialEntries: [cfg.routes.instances],
  });

  const customAcl = makeAcl({
    instances: {
      ...defaultAccess,
      read: hasInstanceReadPermission,
      list: hasInstanceListPermission,
    },
    botInstances: {
      ...defaultAccess,
      read: hasBotInstanceReadPermission,
      list: hasBotInstanceListPermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  ctx.storeUser.state.cluster.authVersion = '18.2.4';

  return (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <TeleportProviderBasic teleportCtx={ctx}>
          <Router history={history}>
            <Route path={cfg.routes.instances}>
              <Instances />
            </Route>
          </Router>
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
