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
import styled from 'styled-components';

import { CardTile } from 'design/CardTile/CardTile';
import Flex from 'design/Flex/Flex';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  listBotInstancesError,
  listBotInstancesForever,
  listBotInstancesSuccess,
} from 'teleport/test/helpers/botInstances';

import { InstancesPanel } from './InstancesPanel';

const meta = {
  title: 'Teleport/Bots/Details/InstancesPanel',
  component: Wrapper,
  beforeEach: () => {
    queryClient.clear(); // Prevent cached data sharing between stories
  },
  excludeStories: ['listBotInstancesSuccessHandler'],
} satisfies Meta<typeof Wrapper>;

type Story = StoryObj<typeof meta>;

export default meta;

export const listBotInstancesSuccessHandler = listBotInstancesSuccess({
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
});

export const Happy: Story = {
  parameters: {
    msw: {
      handlers: [listBotInstancesSuccessHandler],
    },
  },
};

export const WithFetchPending: Story = {
  parameters: {
    msw: {
      handlers: [listBotInstancesForever()],
    },
  },
};

export const WithFetchFailure: Story = {
  parameters: {
    msw: {
      handlers: [listBotInstancesError(500, 'something went wrong')],
    },
  },
};

export const NoBotInstancesListPermission: Story = {
  args: {
    hasBotInstanceListPermission: false,
  },
};

function Wrapper(props: { hasBotInstanceListPermission?: boolean }) {
  const { hasBotInstanceListPermission = true } = props;

  const customAcl = makeAcl({
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
      <TeleportProviderBasic teleportCtx={ctx}>
        <Container>
          <InnerContainer>
            <InstancesPanel botName="ansible-worker" />
          </InnerContainer>
        </Container>
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}

const Container = styled(Flex)`
  align-items: center;
  justify-content: center;
`;

const InnerContainer = styled(CardTile)`
  max-width: 400px;
  height: 600px;
  overflow: auto;
  padding: 0;
  gap: 0;
  margin: ${props => props.theme.space[1]}px;
`;

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: false,
    },
  },
});
