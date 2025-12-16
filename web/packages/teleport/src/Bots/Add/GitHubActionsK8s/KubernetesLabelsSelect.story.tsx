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
import { useState } from 'react';
import styled from 'styled-components';

import Flex from 'design/Flex';
import Validation, { Validator } from 'shared/components/Validation';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  fetchUnifiedResourcesError,
  fetchUnifiedResourcesForever,
  fetchUnifiedResourcesSuccess,
} from 'teleport/test/helpers/resources';
import { userEventCaptureSuccess } from 'teleport/test/helpers/userEvents';

import { KubernetesLabel } from '../Shared/kubernetes';
import { TrackingProvider } from '../Shared/useTracking';
import { KubernetesLabelsSelect } from './KubernetesLabelsSelect';

const meta = {
  title: 'Teleport/Bots/Add/GitHubActions+K8s/KubernetesLabelsSelect',
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
      handlers: [fetchUnifiedResourcesSuccess(), userEventCaptureSuccess()],
    },
  },
};

export const FetchResourcesError: Story = {
  parameters: {
    msw: {
      handlers: [
        fetchUnifiedResourcesError(500, 'something went wrong'),
        userEventCaptureSuccess(),
      ],
    },
  },
};

export const FetchResourcesForever: Story = {
  parameters: {
    msw: {
      handlers: [fetchUnifiedResourcesForever(), userEventCaptureSuccess()],
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
  const [labels, setLabels] = useState<KubernetesLabel[]>([
    { name: '*', values: ['*'] },
  ]);

  const handleLabelsChanged = (
    labels: KubernetesLabel[],
    validator: Validator
  ) => {
    setLabels(labels);
    validator.validate();
  };

  const customAcl = makeAcl({
    kubeServers: {
      ...defaultAccess,
      read: true,
      list: true,
    },
  });

  const ctx = createTeleportContext({
    customAcl,
  });

  return (
    <QueryClientProvider client={queryClient}>
      <TeleportProviderBasic teleportCtx={ctx}>
        <TrackingProvider>
          <Validation>
            {({ validator }) => (
              <Container>
                <KubernetesLabelsSelect
                  selected={labels}
                  onChange={labels => handleLabelsChanged(labels, validator)}
                />
              </Container>
            )}
          </Validation>
        </TrackingProvider>
      </TeleportProviderBasic>
    </QueryClientProvider>
  );
}

const Container = styled(Flex)`
  flex-direction: column;
  gap: ${({ theme }) => theme.space[2]}px;
`;
