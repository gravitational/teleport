/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { ButtonBorder, Flex } from 'design';

import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import { UrlResourcesParams } from 'teleport/config';
import { ResourcesResponse } from 'teleport/services/agents';

import { UnifiedResources } from './UnifiedResources';
import { SharedUnifiedResource } from './types';

export default {
  title: 'Shared/UnifiedResources',
};

const aLotOfLabels = {
  ...databases[0],
  name: 'A DB with a lot of labels',
  labels: Array(300)
    .fill(0)
    .map((_, i) => ({ name: `label-${i}`, value: `value ${i}` })),
};

const allResources = [
  ...apps,
  aLotOfLabels,
  ...databases,
  ...kubes,
  ...desktops,
  ...nodes,
  ...apps,
  ...databases,
  ...kubes,
  ...desktops,
  ...nodes,
].map(resource => ({
  resource,
  ui: {
    ActionButton: <ButtonBorder size="small">Connect</ButtonBorder>,
  },
}));
const story = (
  fetchFunc: (
    params: UrlResourcesParams,
    signal: AbortSignal
  ) => Promise<ResourcesResponse<SharedUnifiedResource>>
) => {
  return () => (
    <UnifiedResources
      availableKinds={['app', 'db', 'node', 'kube_cluster', 'windows_desktop']}
      Header={pinAllButton => (
        <Flex justifyContent="end" height="50px">
          {pinAllButton}
        </Flex>
      )}
      params={{ sort: { dir: 'ASC', fieldName: 'name' } }}
      setParams={() => undefined}
      updateUnifiedResourcesPreferences={() => undefined}
      pinningNotSupported={false}
      getClusterPinnedResources={async () => []}
      onLabelClick={() => undefined}
      EmptySearchResults={undefined}
      updateClusterPinnedResources={() => undefined}
      fetchFunc={fetchFunc}
    />
  );
};

export const Empty = story(async () => ({ agents: [], startKey: '' }));

export const List = story(async () => ({
  agents: allResources,
}));

export const Loading = story(
  (_, signal) =>
    new Promise<never>((resolve, reject) => {
      signal.addEventListener('abort', reject);
    })
);

export const LoadingAfterScrolling = story(async params => {
  if (params.startKey === 'next-key') {
    return new Promise(() => {});
  }
  return {
    agents: allResources,
    startKey: 'next-key',
  };
});

export const Errored = story(async () => {
  throw new Error('Failed to fetch');
});

export const ErroredAfterScrolling = story(async params => {
  if (params.startKey === 'next-key') {
    throw new Error('Failed to fetch');
  }
  return { agents: allResources, startKey: 'next-key' };
});
