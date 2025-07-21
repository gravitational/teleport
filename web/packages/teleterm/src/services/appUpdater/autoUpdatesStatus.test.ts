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

import Logger, { NullService } from 'teleterm/logger';

import { resolveAutoUpdatesStatus } from './autoUpdatesStatus';

beforeAll(() => {
  Logger.init(new NullService());
});

test.each([
  {
    title: 'disabled when env var is "off"',
    input: {
      versionEnvVar: 'off',
      managingClusterUri: '',
      getClusterVersions: async () => ({
        reachableClusters: [],
        unreachableClusters: [],
      }),
    },
    expected: {
      enabled: false,
      reason: 'disabled-by-env-var',
    },
  },
  {
    title: 'resolving with env var when set to version',
    input: {
      versionEnvVar: '14.0.0',
      managingClusterUri: '',
      getClusterVersions: async () => ({
        reachableClusters: [],
        unreachableClusters: [],
      }),
    },
    expected: {
      enabled: true,
      version: '14.0.0',
      source: { kind: 'env-var' },
    },
  },
  {
    title: 'disabled when no versions with auto-update enabled',
    input: {
      versionEnvVar: '',
      managingClusterUri: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: false,
          },
        ],
        unreachableClusters: [],
      }),
    },
    expected: {
      enabled: false,
      reason: 'no-cluster-with-auto-update',
      candidateClusters: [
        {
          clusterUri: '/clusters/cluster-a',
          toolsAutoUpdate: false,
          toolsVersion: '14.0.0',
          minToolsVersion: '13.0.0',
          otherCompatibleClusters: [],
        },
      ],
      unreachableClusters: [],
    },
  },
  {
    title: 'resolving with managing cluster if specified',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '/clusters/cluster-a',
    },
    expected: {
      enabled: true,
      version: '14.0.0',
      source: {
        kind: 'managing-cluster',
        clusterUri: '/clusters/cluster-a',
        candidateClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            otherCompatibleClusters: [],
          },
        ],
        unreachableClusters: [],
      },
    },
  },
  {
    title:
      'resolving using most compatible version when clusters are on the same version',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '/clusters/missing-cluster',
    },
    expected: {
      enabled: true,
      version: '14.0.0',
      source: {
        kind: 'most-compatible',
        candidateClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            otherCompatibleClusters: ['/clusters/cluster-b'],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        clustersUri: ['/clusters/cluster-a', '/clusters/cluster-b'],
      },
    },
  },
  {
    title:
      'resolving using most compatible version when clusters are on the same major version',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '14.1.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '14.2.0',
            minToolsVersion: '13.0.0',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '/clusters/missing-cluster',
    },
    expected: {
      enabled: true,
      version: '14.2.0', // the newer version should be taken
      source: {
        kind: 'most-compatible',
        candidateClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.1.0',
            minToolsVersion: '13.0.0',
            otherCompatibleClusters: ['/clusters/cluster-b'],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '14.2.0',
            minToolsVersion: '13.0.0',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        clustersUri: ['/clusters/cluster-b'],
      },
    },
  },
  {
    title:
      'resolving using most compatible version where clusters are on different major version',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),

      managingClusterUri: '/clusters/missing-cluster',
    },
    expected: {
      enabled: true,
      version: '15.0.0',
      source: {
        kind: 'most-compatible',
        candidateClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0',
            otherCompatibleClusters: [],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        clustersUri: ['/clusters/cluster-b'],
      },
    },
  },
  {
    title:
      'resolving with no compatible version when clusters are on incompatibles versions',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '18.0.0',
            minToolsVersion: '17.0.0',
            toolsAutoUpdate: false,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-c',
            toolsVersion: '16.1.0',
            minToolsVersion: '15.0.0',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '',
    },
    expected: {
      enabled: false,
      reason: 'no-compatible-version',
      candidateClusters: [
        {
          clusterUri: '/clusters/cluster-a',
          toolsAutoUpdate: false,
          toolsVersion: '18.0.0',
          minToolsVersion: '17.0.0',
          otherCompatibleClusters: [],
        },
        {
          clusterUri: '/clusters/cluster-b',
          toolsAutoUpdate: true,
          toolsVersion: '16.0.0',
          minToolsVersion: '15.0.0',
          otherCompatibleClusters: ['/clusters/cluster-c'],
        },
        {
          clusterUri: '/clusters/cluster-c',
          toolsAutoUpdate: true,
          toolsVersion: '16.1.0',
          minToolsVersion: '15.0.0',
          otherCompatibleClusters: ['/clusters/cluster-b'],
        },
      ],
      unreachableClusters: [],
    },
  },
])('$title', async ({ input, expected }) => {
  const result = await resolveAutoUpdatesStatus(input);
  expect(result).toEqual(expected);
});
