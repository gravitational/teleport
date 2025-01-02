/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import * as teleport from 'teleport';
import { getOSSFeatures } from 'teleport/features';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';

import { ClusterListPage } from './Clusters';
import * as fixtures from './fixtures';

export default {
  title: 'Teleport/Clusters',
  excludeStories: ['createContext'],
};

export function Story({ value }: { value: teleport.Context }) {
  const ctx = value || createContext();
  return (
    <teleport.ContextProvider ctx={ctx}>
      <FeaturesContextProvider value={getOSSFeatures()}>
        <Router history={createMemoryHistory()}>
          <ClusterListPage />
        </Router>
      </FeaturesContextProvider>
    </teleport.ContextProvider>
  );
}

Story.storyName = 'Clusters';

export function createContext() {
  const ctx = new teleport.Context();
  ctx.clusterService.fetchClusters = () => Promise.resolve(fixtures.clusters);
  return ctx;
}
