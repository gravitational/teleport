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
import { Route, Routes } from 'react-router';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  listInstancesError,
  listInstancesLoading,
  listInstancesSuccess,
  listOnlyBotInstances,
  listOnlyRegularInstances,
} from 'teleport/test/helpers/instances';

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

export const Loaded: Story = {
  beforeEach({ msw }) {
    msw.use(listInstancesSuccess);
  },
};

export const CacheInitializing: Story = {
  beforeEach({ msw }) {
    msw.use(
      listInstancesError(
        503,
        'inventory cache is not yet healthy, please try again in a few minutes'
      )
    );
  },
};

export const Loading: Story = {
  beforeEach({ msw }) {
    msw.use(listInstancesLoading);
  },
};

export const Error: Story = {
  beforeEach({ msw }) {
    msw.use(listInstancesError(500, 'some error'));
  },
};

export const NoInstancePermissions: Story = {
  args: {
    hasInstanceListPermission: false,
    hasInstanceReadPermission: false,
  },

  beforeEach({ msw }) {
    msw.use(listOnlyBotInstances);
  },
};

export const NoBotInstancePermissions: Story = {
  args: {
    hasBotInstanceListPermission: false,
    hasBotInstanceReadPermission: false,
  },

  beforeEach({ msw }) {
    msw.use(listOnlyRegularInstances);
  },
};

export const NoPermissionsAtAll: Story = {
  args: {
    hasInstanceListPermission: false,
    hasInstanceReadPermission: false,
    hasBotInstanceListPermission: false,
    hasBotInstanceReadPermission: false,
  },

  beforeEach({ msw }) {
    msw.use(listInstancesError(403, 'access denied'));
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
      <TeleportProviderBasic
        teleportCtx={ctx}
        initialEntries={[cfg.routes.instances]}
      >
        <Routes>
          <Route path={cfg.routes.instances} element={<Instances />} />
        </Routes>
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}
