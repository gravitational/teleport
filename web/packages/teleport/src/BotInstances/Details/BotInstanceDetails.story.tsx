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

import { CardTile } from 'design/CardTile';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  getBotInstanceError,
  getBotInstanceForever,
  getBotInstanceSuccess,
} from 'teleport/test/helpers/botInstances';

import { BotInstanceDetails } from './BotInstanceDetails';

const meta = {
  title: 'Teleport/BotInstances/Details',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [
        getBotInstanceSuccess({
          bot_instance: {
            spec: {
              instance_id: 'a55259e8-9b17-466f-9d37-ab390ca4024e',
            },
          },
          yaml: 'kind: bot_instance\nversion: v1\n',
        }),
      ],
    },
  },
};

export const ErrorLoadingList: Story = {
  parameters: {
    msw: {
      handlers: [getBotInstanceError(500, 'something went wrong')],
    },
  },
};

export const StillLoadingList: Story = {
  parameters: {
    msw: {
      handlers: [getBotInstanceForever()],
    },
  },
};

export const NoReadPermission: Story = {
  args: {
    hasBotInstanceReadPermission: false,
  },
  parameters: {
    msw: {
      handlers: [getBotInstanceError(500, 'this call should never be made')],
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

function Wrapper(props?: { hasBotInstanceReadPermission?: boolean }) {
  const { hasBotInstanceReadPermission = true } = props ?? {};

  const customAcl = makeAcl({
    botInstances: {
      ...defaultAccess,
      read: hasBotInstanceReadPermission,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <TeleportProviderBasic teleportCtx={ctx}>
        <CardTile height={820} overflow={'auto'} p={0}>
          <BotInstanceDetails
            botName="ansible-worker"
            instanceId="a55259e8-9b17-466f-9d37-ab390ca4024e"
            onClose={() => {}}
          />
        </CardTile>
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}
