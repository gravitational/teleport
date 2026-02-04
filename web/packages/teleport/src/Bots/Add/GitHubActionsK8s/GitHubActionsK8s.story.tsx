/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';
import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import { BotFlowType } from 'teleport/Bots/types';
import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { genWizardCiCdSuccess } from 'teleport/test/helpers/bots';
import { fetchUnifiedResourcesSuccess } from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { GitHubActionsK8s } from './GitHubActionsK8s';

const meta = {
  title: 'Teleport/Bots/Add/GitHubActions+K8s/GitHubActionsK8s',
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
        genWizardCiCdSuccess({ prettyFormat: true }),
        fetchUnifiedResourcesSuccess({
          delayMs: 1000,
          mockSearch: true,
        }),
        userEventCaptureSuccess(),
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

function Wrapper() {
  const ctx = createTeleportContext();

  return (
    <QueryClientProvider client={queryClient}>
      <ContextProvider ctx={ctx}>
        <InfoGuidePanelProvider>
          <ContentMinWidth>
            <MemoryRouter
              initialEntries={[
                cfg.getBotsNewRoute(BotFlowType.GitHubActionsK8s),
              ]}
            >
              <Container>
                <GitHubActionsK8s />
              </Container>
            </MemoryRouter>
          </ContentMinWidth>
        </InfoGuidePanelProvider>
      </ContextProvider>
    </QueryClientProvider>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  height: 820px;
  overflow: auto;
`;
