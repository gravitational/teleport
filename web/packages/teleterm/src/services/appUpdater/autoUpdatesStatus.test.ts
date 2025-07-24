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

import {
  AutoUpdatesEnabled,
  resolveAutoUpdatesStatus,
  shouldAutoDownload,
} from './autoUpdatesStatus';

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
            minToolsVersion: '13.0.0-aa',
            toolsAutoUpdate: false,
          },
        ],
        unreachableClusters: [],
      }),
    },
    expected: {
      enabled: false,
      reason: 'no-cluster-with-auto-update',
      clusters: [
        {
          clusterUri: '/clusters/cluster-a',
          toolsAutoUpdate: false,
          toolsVersion: '14.0.0',
          minToolsVersion: '13.0.0-aa',
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
            minToolsVersion: '13.0.0-aa',
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
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0-aa',
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
            minToolsVersion: '13.0.0-aa',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0-aa',
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
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-b'],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '14.0.0',
            minToolsVersion: '13.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        skippedManagingClusterUri: '',
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
            minToolsVersion: '13.0.0-aa',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '14.2.0',
            minToolsVersion: '13.0.0-aa',
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
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '14.1.0',
            minToolsVersion: '13.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-b'],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '14.2.0',
            minToolsVersion: '13.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        skippedManagingClusterUri: '',
      },
    },
  },
  {
    title:
      'resolving to stable version when both stable and pre-release are available',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '18.2.0-beta.1',
            minToolsVersion: '18.0.0-aa',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '18.2.0',
            minToolsVersion: '18.0.0-aa',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '',
    },
    expected: {
      enabled: true,
      version: '18.2.0', // From semver spec: pre-releases have lower precedence than a normal version.
      source: {
        kind: 'most-compatible',
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '18.2.0-beta.1',
            minToolsVersion: '18.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-b'],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '18.2.0',
            minToolsVersion: '18.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        skippedManagingClusterUri: '',
      },
    },
  },
  {
    title:
      'resolving using most compatible version when clusters are on different major version',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
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
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: true,
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            otherCompatibleClusters: [],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        unreachableClusters: [],
        skippedManagingClusterUri: '',
      },
    },
  },
  {
    title:
      'resolving using most compatible version when cluster managing no longer has `toolsAutoUpdate` set to true',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            toolsAutoUpdate: false,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [],
      }),
      managingClusterUri: '/clusters/cluster-a',
    },
    expected: {
      enabled: true,
      version: '15.0.0',
      source: {
        kind: 'most-compatible',
        clusters: [
          {
            clusterUri: '/clusters/cluster-a',
            toolsAutoUpdate: false,
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            otherCompatibleClusters: [],
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
            otherCompatibleClusters: ['/clusters/cluster-a'],
          },
        ],
        skippedManagingClusterUri: '/clusters/cluster-a',
        unreachableClusters: [],
      },
    },
  },
  {
    title:
      'resolving using most compatible version when cluster managing updates is unreachable',
    input: {
      versionEnvVar: '',
      getClusterVersions: async () => ({
        reachableClusters: [
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
            toolsAutoUpdate: true,
          },
        ],
        unreachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            errorMessage: 'Something went wrong',
          },
        ],
      }),
      managingClusterUri: '/clusters/cluster-a',
    },
    expected: {
      enabled: true,
      version: '15.0.0',
      source: {
        kind: 'most-compatible',
        clusters: [
          {
            clusterUri: '/clusters/cluster-b',
            toolsAutoUpdate: true,
            toolsVersion: '15.0.0',
            minToolsVersion: '14.0.0-aa',
            otherCompatibleClusters: [],
          },
        ],
        skippedManagingClusterUri: '/clusters/cluster-a',
        unreachableClusters: [
          {
            clusterUri: '/clusters/cluster-a',
            errorMessage: 'Something went wrong',
          },
        ],
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
            minToolsVersion: '17.0.0-aa',
            toolsAutoUpdate: false,
          },
          {
            clusterUri: '/clusters/cluster-b',
            toolsVersion: '16.0.0',
            minToolsVersion: '15.0.0-aa',
            toolsAutoUpdate: true,
          },
          {
            clusterUri: '/clusters/cluster-c',
            toolsVersion: '16.1.0',
            minToolsVersion: '15.0.0-aa',
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
      clusters: [
        {
          clusterUri: '/clusters/cluster-a',
          toolsAutoUpdate: false,
          toolsVersion: '18.0.0',
          minToolsVersion: '17.0.0-aa',
          otherCompatibleClusters: [],
        },
        {
          clusterUri: '/clusters/cluster-b',
          toolsAutoUpdate: true,
          toolsVersion: '16.0.0',
          minToolsVersion: '15.0.0-aa',
          otherCompatibleClusters: ['/clusters/cluster-c'],
        },
        {
          clusterUri: '/clusters/cluster-c',
          toolsAutoUpdate: true,
          toolsVersion: '16.1.0',
          minToolsVersion: '15.0.0-aa',
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

describe('should not auto download', () => {
  test.each<{ title: string; input: AutoUpdatesEnabled }>([
    {
      title: 'when cluster previously managing updates was skipped',
      input: {
        enabled: true,
        version: '15.0.0',
        source: {
          clusters: [
            {
              clusterUri: '/clusters/cluster-a',
              minToolsVersion: '15.0.0-aa',
              otherCompatibleClusters: ['/clusters/cluster-b'],
              toolsAutoUpdate: false,
              toolsVersion: '16.1.0',
            },
            {
              clusterUri: '/clusters/cluster-b',
              minToolsVersion: '15.0.0-aa',
              otherCompatibleClusters: ['/clusters/cluster-a'],
              toolsAutoUpdate: true,
              toolsVersion: '16.1.0',
            },
          ],
          skippedManagingClusterUri: '/clusters/cluster-b',
          kind: 'most-compatible',
          unreachableClusters: [],
        },
      },
    },
    {
      title: 'when there is unreachable cluster',
      input: {
        enabled: true,
        version: '16.1.0',
        source: {
          clusters: [
            {
              clusterUri: '/clusters/cluster-b',
              minToolsVersion: '15.0.0-aa',
              otherCompatibleClusters: [],
              toolsAutoUpdate: true,
              toolsVersion: '16.1.0',
            },
          ],
          skippedManagingClusterUri: '/clusters/cluster-b',
          kind: 'most-compatible',
          unreachableClusters: [
            {
              clusterUri: '/clusters/cluster-a',
              errorMessage: 'Something went wrong',
            },
          ],
        },
      },
    },
  ])('$title', ({ input }) => {
    const result = shouldAutoDownload(input);
    expect(result).toBeFalsy();
  });
});
