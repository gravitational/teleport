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

import { MemoryRouter } from 'react-router';

import { Route } from 'teleport/components/Router';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { clusterInfoFixture } from '../fixtures';
import { ManageCluster } from './ManageCluster';

export default {
  title: 'Teleport/Clusters/ManageCluster',
};

function render(fetchClusterDetails: (clusterId: string) => Promise<any>) {
  const ctx = createTeleportContext();

  ctx.clusterService.fetchClusterDetails = fetchClusterDetails;
  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>
            <ManageCluster />
          </ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

export function Loading() {
  const fetchClusterDetails = () => {
    // promise never resolves to simulate loading state
    return new Promise(() => {});
  };
  return render(fetchClusterDetails);
}

export function Failed() {
  const fetchClusterDetails = () =>
    Promise.reject(new Error('Failed to load cluster details'));
  return render(fetchClusterDetails);
}

export function Success() {
  const fetchClusterDetails = () => {
    return new Promise(resolve => {
      resolve(clusterInfoFixture);
    });
  };
  return render(fetchClusterDetails);
}
