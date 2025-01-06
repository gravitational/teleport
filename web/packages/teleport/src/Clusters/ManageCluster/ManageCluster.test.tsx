/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter, Route } from 'react-router-dom';

import { render, screen, waitFor } from 'design/utils/testing';

import cfg from 'teleport/config';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { clusterInfoFixture } from '../fixtures';
import { ManageCluster } from './ManageCluster';

function renderElement(element, ctx) {
  return render(
    <MemoryRouter initialEntries={[`/clusters/cluster-id`]}>
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>{element}</ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

describe('test ManageCluster component', () => {
  const server = setupServer(
    http.get(cfg.getClusterInfoPath('cluster-id'), () => {
      return HttpResponse.json({
        name: 'cluster-id',
        lastConnected: new Date(),
        status: 'active',
        publicURL: 'cluster-id.teleport.com',
        authVersion: 'v17.0.0',
        proxyVersion: 'v17.0.0',
        isCloud: false,
        licenseExpiry: new Date(),
      });
    })
  );

  beforeAll(() => server.listen());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  test('fetches cluster information on load', async () => {
    const ctx = createTeleportContext();

    renderElement(<ManageCluster />, ctx);
    await waitFor(() => {
      expect(screen.getByText('v17.0.0')).toBeInTheDocument();
    });

    expect(screen.getByText('cluster-id')).toBeInTheDocument();
    expect(screen.getByText('cluster-id.teleport.com')).toBeInTheDocument();
  });

  test('shows error when load fails', async () => {
    server.use(
      http.get(cfg.getClusterInfoPath('cluster-id'), () => {
        return HttpResponse.json(
          {
            message: 'Failed to load cluster information',
          },
          { status: 400 }
        );
      })
    );

    const ctx = createTeleportContext();

    renderElement(<ManageCluster />, ctx);
    await waitFor(() => {
      expect(
        screen.getByText('Failed to load cluster information')
      ).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(
        screen.queryByText(clusterInfoFixture.authVersion)
      ).not.toBeInTheDocument();
    });
  });
});
