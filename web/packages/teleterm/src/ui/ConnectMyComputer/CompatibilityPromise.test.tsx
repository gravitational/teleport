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

import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';

import {
  checkAgentCompatibility,
  CompatibilityError,
} from './CompatibilityPromise';

describe('isAgentCompatible', () => {
  const testCases = [
    {
      agentVersion: '2.0.0',
      proxyVersion: '2.0.0',
      expected: 'compatible',
    },
    {
      agentVersion: '2.1.0',
      proxyVersion: '2.0.0',
      expected: 'compatible',
    },
    {
      agentVersion: '3.0.0',
      proxyVersion: '2.0.0',
      expected: 'incompatible',
    },
    {
      agentVersion: '2.0.0',
      proxyVersion: '3.0.0',
      expected: 'compatible',
    },
    {
      agentVersion: '2.0.0',
      proxyVersion: '4.0.0',
      expected: 'incompatible',
    },
    {
      agentVersion: '2.0.0',
      proxyVersion: '',
      expected: 'unknown',
    },
  ];
  test.each(testCases)(
    'should agent $agentVersion and cluster $proxyVersion be compatible? $expected',
    ({ agentVersion, proxyVersion, expected }) => {
      expect(
        checkAgentCompatibility(
          proxyVersion,
          makeRuntimeSettings({ appVersion: agentVersion })
        )
      ).toBe(expected);
    }
  );
});

test('compatibilityError shows app upgrade instructions', async () => {
  const agentVersion = '1.0.0';
  const proxyVersion = '3.0.0';
  const appContext = new MockAppContext({ appVersion: agentVersion });
  const cluster = makeRootCluster({ proxyVersion });
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <CompatibilityError />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  await expect(
    screen.findByText(
      /The cluster is on version 3.0.0 while Teleport Connect is on version 1.0.0./
    )
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/upgrade the app to 3.x.x/)
  ).resolves.toBeVisible();
});

test('compatibilityError shows cluster upgrade (and app downgrade) instructions', async () => {
  const agentVersion = '16.0.0';
  const proxyVersion = '15.0.0';
  const appContext = new MockAppContext({ appVersion: agentVersion });
  const cluster = makeRootCluster({ proxyVersion });
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <CompatibilityError />
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  await expect(
    screen.findByText(
      /The cluster is on version 15.0.0 while Teleport Connect is on version 16.0.0./
    )
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/downgrade the app to version 15.x.x/)
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/upgrade the cluster to version 16.x.x/)
  ).resolves.toBeVisible();
});
