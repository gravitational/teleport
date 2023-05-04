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
import { renderHook } from '@testing-library/react-hooks';

import { ServerUri } from 'teleterm/ui/uri';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { SearchResult } from 'teleterm/ui/services/resources';
import {
  makeServer,
  makeKube,
  makeLabelsList,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';

import { MockAppContextProvider } from '../fixtures/MockAppContextProvider';

import { makeResourceResult } from './testHelpers';
import { rankResults, useFilterSearch, useResourceSearch } from './useSearch';

import type * as tsh from 'teleterm/services/tshd/types';

beforeEach(() => {
  jest.restoreAllMocks();
});

describe('rankResults', () => {
  it('uses the displayed resource name as the tie breaker if the scores are equal', () => {
    const server = makeResourceResult({
      kind: 'server',
      resource: makeServer({ hostname: 'z' }),
    });
    const kube = makeResourceResult({
      kind: 'kube',
      resource: makeKube({ name: 'a' }),
    });
    const sortedResults = rankResults([server, kube], '');

    expect(sortedResults[0]).toEqual(kube);
    expect(sortedResults[1]).toEqual(server);
  });

  it('saves individual label match scores', () => {
    const server = makeResourceResult({
      kind: 'server',
      resource: makeServer({
        labelsList: makeLabelsList({ quux: 'bar-baz', foo: 'bar' }),
      }),
    });

    const { labelMatches } = rankResults([server], 'foo bar')[0];

    labelMatches.forEach(match => {
      expect(match.score).toBeGreaterThan(0);
    });

    const quuxMatches = labelMatches.filter(
      match => match.labelName === 'quux'
    );
    const quuxMatch = quuxMatches[0];
    const fooMatches = labelMatches.filter(match => match.labelName === 'foo');

    expect(quuxMatches).toHaveLength(1);
    expect(fooMatches).toHaveLength(2);
    expect(fooMatches[0].score).toBeGreaterThan(quuxMatch.score);
    expect(fooMatches[1].score).toBeGreaterThan(quuxMatch.score);
  });

  it('limits the results', () => {
    const servers = Array(10)
      .fill(undefined)
      .map(() =>
        makeResourceResult({
          kind: 'server',
          resource: makeServer({
            labelsList: makeLabelsList({ foo: 'bar1' }),
          }),
        })
      );

    // This item has the lowest score, added as the first one
    const lowestScoreServerUri: ServerUri = '/clusters/test/servers/lowest';
    servers.unshift(
      makeResourceResult({
        kind: 'server',
        resource: makeServer({
          uri: lowestScoreServerUri,
          labelsList: makeLabelsList({ foo: 'bar123456' }),
        }),
      })
    );

    // This item has the highest score, added as the last one
    const highestScoreServerUri: ServerUri = '/clusters/test/servers/highest';
    servers.push(
      makeResourceResult({
        kind: 'server',
        resource: makeServer({
          uri: highestScoreServerUri,
          labelsList: makeLabelsList({ foo: 'bar' }),
        }),
      })
    );

    const sorted = rankResults(servers, 'bar');

    expect(sorted).toHaveLength(10);
    // the item with the highest score is the first one
    expect(sorted[0].resource.uri).toBe(highestScoreServerUri);
    // the item with the lowest score is not included
    expect(
      sorted.find(s => s.resource.uri === lowestScoreServerUri)
    ).toBeFalsy();
  });
});

describe('useResourceSearch', () => {
  it('does not limit results returned by ResourcesService', async () => {
    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });
    const servers: SearchResult[] = Array(20)
      .fill(undefined)
      .map(() => ({
        kind: 'server' as const,
        resource: makeServer({}),
      }));
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue([{ status: 'fulfilled', value: servers }]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const searchResult = await result.current('foo', []);

    expect(searchResult.results).toEqual(servers);
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      search: 'foo',
      filter: undefined,
      limit: 100,
    });
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledTimes(
      1
    );
  });

  it('fetches only a preview if search is empty and there is at least one filter selected', async () => {
    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue([{ status: 'fulfilled', value: [] }]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const filter = { filter: 'cluster' as const, clusterUri: cluster.uri };
    await result.current('', [filter]);

    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      search: '',
      filter: undefined,
      limit: 5,
    });
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledTimes(
      1
    );
  });

  it('does not fetch any resources if search is empty and there are no filters selected', async () => {
    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue([{ status: 'fulfilled', value: [] }]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    await result.current('', []);
    expect(appContext.resourcesService.searchResources).not.toHaveBeenCalled();
  });
});

describe('useFiltersSearch', () => {
  it('does not return cluster filters if there is only one cluster', () => {
    const appContext = new MockAppContext();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set('/clusters/teleport-local', {
        connected: true,
        authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
        leaf: false,
        name: 'teleport-local',
        proxyHost: 'localhost:3080',
        uri: '/clusters/teleport-local',
      });
    });

    const { result } = renderHook(() => useFilterSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const clusterFilters = result
      .current('', [])
      .filter(f => f.kind === 'cluster-filter');
    expect(clusterFilters).toHaveLength(0);
  });

  it('returns one cluster filter if the search term matches it', () => {
    const appContext = new MockAppContext();
    const clusterA: tsh.Cluster = {
      connected: true,
      authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
      leaf: false,
      name: 'teleport-a',
      proxyHost: 'localhost:3080',
      uri: '/clusters/teleport-a',
    };
    const clusterB: tsh.Cluster = {
      connected: true,
      authClusterId: '73c4746b-d956-4f16-1848-4e3469f70762',
      leaf: false,
      name: 'teleport-b',
      proxyHost: 'localhost:3080',
      uri: '/clusters/teleport-b',
    };
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(clusterA.uri, clusterA);
      draftState.clusters.set(clusterB.uri, clusterB);
    });

    const { result } = renderHook(() => useFilterSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const clusterFilters = result
      .current('teleport-a', [])
      .filter(f => f.kind === 'cluster-filter');
    expect(clusterFilters).toHaveLength(1);
    expect(clusterFilters[0].resource).toEqual(clusterA);
  });
});
