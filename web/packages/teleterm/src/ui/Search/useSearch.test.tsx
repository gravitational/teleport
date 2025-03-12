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

import { renderHook } from '@testing-library/react';

import { ShowResources } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import {
  makeKube,
  makeLabelsList,
  makeLeafCluster,
  makeRootCluster,
  makeServer,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { SearchResult } from 'teleterm/ui/services/resources';
import { ServerUri } from 'teleterm/ui/uri';

import { MockAppContextProvider } from '../fixtures/MockAppContextProvider';
import { makeResourceResult } from './testHelpers';
import { rankResults, useFilterSearch, useResourceSearch } from './useSearch';

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

  it('prefers accessible resources over requestable ones', () => {
    const serverAccessible = makeResourceResult({
      kind: 'server',
      resource: makeServer({ hostname: 'sales-foo' }),
    });
    const serverRequestable = makeResourceResult({
      kind: 'server',
      resource: makeServer({ hostname: 'sales-bar' }),
      requiresRequest: true,
    });
    const labelMatch = makeResourceResult({
      kind: 'server',
      resource: makeServer({
        hostname: 'lorem-ipsum',
        labels: makeLabelsList({ foo: 'sales' }),
      }),
    });
    const sortedResults = rankResults(
      [labelMatch, serverRequestable, serverAccessible],
      'sales'
    );

    expect(sortedResults[0].resource).toEqual(serverAccessible.resource);
    expect(sortedResults[1].resource).toEqual(serverRequestable.resource);
    expect(sortedResults[2].resource).toEqual(labelMatch.resource);
  });

  it('saves individual label match scores', () => {
    const server = makeResourceResult({
      kind: 'server',
      resource: makeServer({
        labels: makeLabelsList({ quux: 'bar-baz', foo: 'bar' }),
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
            labels: makeLabelsList({ foo: 'bar1' }),
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
          labels: makeLabelsList({ foo: 'bar123456' }),
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
          labels: makeLabelsList({ foo: 'bar' }),
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
        requiresRequest: false,
      }));
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue(servers);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const searchResult = await result.current('foo', [], false);

    expect(searchResult.results).toEqual(servers);
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      search: 'foo',
      filters: [],
      limit: 100,
      includeRequestable: false,
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
      .mockResolvedValue([]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const filter = { filter: 'cluster' as const, clusterUri: cluster.uri };
    await result.current('', [filter], false);

    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: cluster.uri,
      search: '',
      filters: [],
      limit: 10,
      includeRequestable: false,
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
      .mockResolvedValue([]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    await result.current('', [], false);
    expect(appContext.resourcesService.searchResources).not.toHaveBeenCalled();
  });

  it('does not fetch any resources if advanced search is enabled', async () => {
    const appContext = new MockAppContext();
    const cluster = makeRootCluster();
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue([]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    await result.current('foo', [], true);
    expect(appContext.resourcesService.searchResources).not.toHaveBeenCalled();
  });

  it('fetches requestable resources for leaves if the root cluster allows it', async () => {
    const appContext = new MockAppContext();
    const rootCluster = makeRootCluster({
      showResources: ShowResources.REQUESTABLE,
      features: { advancedAccessWorkflows: true, isUsageBasedBilling: false },
    });
    const leafCluster = makeLeafCluster({
      showResources: ShowResources.UNSPECIFIED,
    });
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(rootCluster.uri, rootCluster);
      draftState.clusters.set(leafCluster.uri, leafCluster);
    });
    jest
      .spyOn(appContext.resourcesService, 'searchResources')
      .mockResolvedValue([]);

    const { result } = renderHook(() => useResourceSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    await result.current('foo', [], false);
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledTimes(
      2
    );
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: rootCluster.uri,
      filters: [],
      includeRequestable: true,
      limit: 100,
      search: 'foo',
    });
    expect(appContext.resourcesService.searchResources).toHaveBeenCalledWith({
      clusterUri: leafCluster.uri,
      filters: [],
      includeRequestable: true,
      limit: 100,
      search: 'foo',
    });
  });
});

describe('useFiltersSearch', () => {
  it('resource type filter is matched by the readable name', () => {
    const appContext = new MockAppContext();
    appContext.clustersService.setState(draftState => {
      const rootCluster = makeRootCluster();
      draftState.clusters.set(rootCluster.uri, rootCluster);
    });

    const { result } = renderHook(() => useFilterSearch(), {
      wrapper: ({ children }) => (
        <MockAppContextProvider appContext={appContext}>
          {children}
        </MockAppContextProvider>
      ),
    });
    const clusterFilters = result.current('serv', []);
    expect(clusterFilters).toEqual([
      {
        kind: 'resource-type-filter',
        resource: 'node',
        nameMatch: 'serv',
        score: 100,
      },
    ]);
  });

  it('does not return cluster filters if there is only one cluster', () => {
    const appContext = new MockAppContext();
    appContext.clustersService.setState(draftState => {
      const rootCluster = makeRootCluster();
      draftState.clusters.set(rootCluster.uri, rootCluster);
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
    const clusterA = makeRootCluster({
      name: 'teleport-a',
      proxyHost: 'localhost:3080',
      uri: '/clusters/teleport-a',
    });
    const clusterB = makeRootCluster({
      name: 'teleport-b',
      proxyHost: 'localhost:3080',
      uri: '/clusters/teleport-b',
    });
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
