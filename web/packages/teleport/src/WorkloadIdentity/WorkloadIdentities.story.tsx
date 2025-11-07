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

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';
import {
  listWorkloadIdentitiesError,
  listWorkloadIdentitiesForever,
  listWorkloadIdentitiesSuccess,
} from 'teleport/test/helpers/workloadIdentities';

import { WorkloadIdentities } from './WorkloadIdentities';

const meta = {
  title: 'Teleport/WorkloadIdentity',
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
      handlers: [listWorkloadIdentitiesSuccess()],
    },
  },
};

export const Empty: Story = {
  parameters: {
    msw: {
      handlers: [
        listWorkloadIdentitiesSuccess({
          items: [],
          next_page_token: null,
        }),
      ],
    },
  },
};

export const NoListPermission: Story = {
  args: { hasListPermission: false },
  parameters: {
    msw: {
      handlers: [
        /* should never make a call */
      ],
    },
  },
};

export const Error: Story = {
  parameters: {
    msw: {
      handlers: [listWorkloadIdentitiesError(500, 'something went wrong')],
    },
  },
};

export const OutdatedProxy: Story = {
  parameters: {
    msw: {
      handlers: [
        listWorkloadIdentitiesError(404, 'path not found', {
          proxyVersion: {
            major: 18,
            minor: 0,
            patch: 0,
            preRelease: '',
            string: '18.0.0',
          },
        }),
      ],
    },
  },
};

export const UnsupportedSort: Story = {
  parameters: {
    msw: {
      handlers: [
        listWorkloadIdentitiesError(
          400,
          'unsupported sort, with some more info'
        ),
      ],
    },
  },
};

export const Loading: Story = {
  parameters: {
    msw: {
      handlers: [listWorkloadIdentitiesForever()],
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

function Wrapper(props?: { hasListPermission?: boolean }) {
  const { hasListPermission = true } = props ?? {};

  const history = createMemoryHistory({
    initialEntries: [cfg.routes.workloadIdentities],
  });

  const customAcl = makeAcl({
    workloadIdentity: {
      ...defaultAccess,
      list: hasListPermission,
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
            <Route path={cfg.routes.workloadIdentities}>
              <WorkloadIdentities />
            </Route>
          </Router>
        </TeleportProviderBasic>
      </MemoryRouter>
    </QueryClientProvider>
  );
}
