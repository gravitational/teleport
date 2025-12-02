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

import Box from 'design/Box/Box';

import {
  genWizardCiCdError,
  genWizardCiCdForever,
  genWizardCiCdSuccess,
} from 'teleport/test/helpers/bots';

import { ConnectGitHub } from './ConnectGitHub';
import { GitHubK8sFlowProvider } from './useGitHubK8sFlow';

const meta = {
  title: 'Teleport/Bots/GHA+K8sWizard/ConnectGitHub',
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
      handlers: [genWizardCiCdSuccess()],
    },
  },
};

export const TemplateFetchFailed: Story = {
  parameters: {
    msw: {
      handlers: [genWizardCiCdError(500, 'something went wrong')],
    },
  },
};

export const TemplateFetching: Story = {
  parameters: {
    msw: {
      handlers: [genWizardCiCdForever()],
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
  return (
    <QueryClientProvider client={queryClient}>
      <GitHubK8sFlowProvider>
        <Box height={820} overflow={'auto'}>
          <ConnectGitHub />
        </Box>
      </GitHubK8sFlowProvider>
    </QueryClientProvider>
  );
}
