/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

import * as teleport from 'teleport';

import { FeaturesContextProvider } from 'teleport/FeaturesContext';

import { getOSSFeatures } from 'teleport/features';

import { Clusters } from './Clusters';
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
          <Clusters />
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
