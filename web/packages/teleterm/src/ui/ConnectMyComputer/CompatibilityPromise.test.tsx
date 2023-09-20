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
import { screen } from '@testing-library/react';
import { render, act } from 'design/utils/testing';

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import {
  isAgentCompatible,
  CompatibilityError,
  UpgradeAgentSuggestion,
} from './CompatibilityPromise';
import { ConnectMyComputerContextProvider } from './connectMyComputerContext';

describe('isAgentCompatible', () => {
  const testCases = [
    {
      agentVersion: '2.0.0',
      proxyVersion: '2.0.0',
      isCompatible: true,
    },
    {
      agentVersion: '2.1.0',
      proxyVersion: '2.0.0',
      isCompatible: true,
    },
    {
      agentVersion: '3.0.0',
      proxyVersion: '2.0.0',
      isCompatible: false,
    },
    {
      agentVersion: '2.0.0',
      proxyVersion: '3.0.0',
      isCompatible: true,
    },
    {
      agentVersion: '2.0.0',
      proxyVersion: '4.0.0',
      isCompatible: false,
    },
  ];
  test.each(testCases)(
    'should agent $agentVersion and cluster $proxyVersion be compatible? $isCompatible',
    ({ agentVersion, proxyVersion, isCompatible }) => {
      expect(
        isAgentCompatible(
          makeRootCluster({ proxyVersion }),
          makeRuntimeSettings({ appVersion: agentVersion })
        )
      ).toBe(isCompatible);
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
  const agentVersion = '15.0.0';
  const proxyVersion = '14.0.0';
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
      /The cluster is on version 14.0.0 while Teleport Connect is on version 15.0.0./
    )
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/downgrade the app to 14.1.0/)
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/upgrade the cluster to version 15.x.x/)
  ).resolves.toBeVisible();
});

test('upgradeAgentSuggestion is visible when the agent is compatible and cluster is older than the agent', async () => {
  const agentVersion = '14.1.0';
  const proxyVersion = '15.0.0';
  const appContext = new MockAppContext({ appVersion: agentVersion });
  const cluster = makeRootCluster({ proxyVersion });
  appContext.clustersService.setState(draftState => {
    draftState.clusters.set(cluster.uri, cluster);
  });

  render(
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
        <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
          <UpgradeAgentSuggestion />
        </ConnectMyComputerContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );

  await expect(
    screen.findByText(/agent is running version 14.1.0 of Teleport/)
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/Consider upgrading it to 15.0.0/)
  ).resolves.toBeVisible();
});

describe('upgradeAgentSuggestion is not visible when', () => {
  const testCases = [
    {
      name: 'the agent is not compatible',
      agentVersion: '15.0.0',
      proxyVersion: '17.0.0',
    },
    {
      name: 'the cluster is already on a newer version',
      agentVersion: '15.0.0',
      proxyVersion: '14.0.0',
    },
  ];
  test.each(testCases)('$name', async ({ agentVersion, proxyVersion }) => {
    const appContext = new MockAppContext({ appVersion: agentVersion });
    const cluster = makeRootCluster({ proxyVersion });
    appContext.clustersService.setState(draftState => {
      draftState.clusters.set(cluster.uri, cluster);
    });

    render(
      <MockAppContextProvider appContext={appContext}>
        <MockWorkspaceContextProvider rootClusterUri={cluster.uri}>
          <ConnectMyComputerContextProvider rootClusterUri={cluster.uri}>
            <UpgradeAgentSuggestion />
          </ConnectMyComputerContextProvider>
        </MockWorkspaceContextProvider>
      </MockAppContextProvider>
    );

    // suppresses 'An update was not wrapped in act(...)'
    await act(() =>
      appContext.connectMyComputerService.isAgentConfigFileCreated(cluster.uri)
    );

    expect(
      screen.queryByText(/agent is running version/)
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Consider upgrading it/)).not.toBeInTheDocument();
  });
});
