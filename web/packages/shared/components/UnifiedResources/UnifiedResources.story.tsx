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

import { useState } from 'react';

import { ButtonBorder } from 'design';
import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  UnifiedResourcePreferences,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { makeErrorAttempt, makeProcessingAttempt } from 'shared/hooks/useAsync';

import { apps, moreApps } from 'teleport/Apps/fixtures';
import { UrlResourcesParams } from 'teleport/config';
import { databases, moreDatabases } from 'teleport/Databases/fixtures';
import { desktops, moreDesktops } from 'teleport/Desktops/fixtures';
import { gitServers } from 'teleport/GitServers/fixtures';
import { kubes, moreKubes } from 'teleport/Kubes/fixtures';
import { moreNodes, nodes } from 'teleport/Nodes/fixtures';
import { ResourcesResponse } from 'teleport/services/agents';

import { SharedUnifiedResource, UnifiedResourcesQueryParams } from './types';
import {
  UnifiedResources,
  UnifiedResourcesProps,
  useUnifiedResourcesFetch,
} from './UnifiedResources';

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
  ...moreApps,
  ...moreDatabases,
  ...moreKubes,
  ...moreDesktops,
  ...moreNodes,
  ...gitServers,
];

const story = ({
  fetchFunc,
  pinning = {
    kind: 'supported',
    getClusterPinnedResources: async () => [],
    updateClusterPinnedResources: async () => undefined,
  },
  params,
  ...props
}: {
  fetchFunc: (
    params: UrlResourcesParams,
    signal: AbortSignal
  ) => Promise<ResourcesResponse<SharedUnifiedResource['resource']>>;
} & Omit<Partial<UnifiedResourcesProps>, 'fetchResources'>) => {
  const mergedParams: UnifiedResourcesQueryParams = {
    ...{
      sort: {
        dir: 'ASC',
        fieldName: 'name',
      },
    },
    ...params,
  };
  return () => {
    const [userPrefs, setUserPrefs] = useState<UnifiedResourcePreferences>({
      defaultTab: DefaultTab.ALL,
      viewMode: ViewMode.CARD,
      labelsViewMode: LabelsViewMode.COLLAPSED,
      availableResourceMode: AvailableResourceMode.ACCESSIBLE,
    });
    const { fetch, attempt, resources } = useUnifiedResourcesFetch({
      fetchFunc,
    });
    return (
      <UnifiedResources
        availableKinds={[
          {
            kind: 'app',
            disabled: false,
          },
          {
            kind: 'db',
            disabled: false,
          },
          {
            kind: 'node',
            disabled: false,
          },
          {
            kind: 'kube_cluster',
            disabled: false,
          },
          {
            kind: 'windows_desktop',
            disabled: false,
          },
        ]}
        params={mergedParams}
        setParams={() => undefined}
        pinning={pinning}
        unifiedResourcePreferences={userPrefs}
        updateUnifiedResourcesPreferences={setUserPrefs}
        NoResources={undefined}
        fetchResources={fetch}
        resourcesFetchAttempt={attempt}
        resources={resources.map(resource => ({
          resource,
          ui: {
            ActionButton: <ButtonBorder size="small">Connect</ButtonBorder>,
          },
        }))}
        {...props}
      />
    );
  };
};

export const Empty = story({
  fetchFunc: async () => ({ agents: [], startKey: '' }),
});

export const List = story({
  fetchFunc: async () => ({
    agents: allResources,
  }),
});

export const NoResults = story({
  fetchFunc: async () => ({
    agents: [],
  }),
  params: { search: 'my super long search query' },
});

export const Loading = story({
  fetchFunc: (_, signal) =>
    new Promise<never>((resolve, reject) => {
      signal.addEventListener('abort', reject);
    }),
});

export const LoadingAfterScrolling = story({
  fetchFunc: async params => {
    if (params.startKey === 'next-key') {
      return new Promise(() => {});
    }
    return {
      agents: allResources,
      startKey: 'next-key',
    };
  },
});

export const Failed = story({
  fetchFunc: async () => {
    throw new Error('Failed to fetch');
  },
});

export const FailedAfterScrolling = story({
  fetchFunc: async params => {
    if (params.startKey === 'next-key') {
      throw new Error('Failed to fetch');
    }
    return { agents: allResources, startKey: 'next-key' };
  },
});

export const FailedToLoadPreferences = story({
  fetchFunc: async () => ({
    agents: allResources,
  }),
  unifiedResourcePreferencesAttempt: makeErrorAttempt(
    new Error('Network error')
  ),
});

export const LoadingPreferences = story({
  fetchFunc: async () => ({
    agents: allResources,
  }),
  unifiedResourcePreferencesAttempt: makeProcessingAttempt(),
});

export const PinningHidden = story({
  fetchFunc: async () => {
    return { agents: allResources, startKey: 'next-key' };
  },
  pinning: { kind: 'hidden' },
});
