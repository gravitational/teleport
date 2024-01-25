/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';
import { MemoryRouter } from 'react-router';
import { rest } from 'msw';
import { mswLoader } from 'msw-storybook-addon';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { ContextProvider } from 'teleport';

import { generateCmd } from 'teleport/Discover/Kubernetes/HelmChart/HelmChart';

import { ManualHelmDialog } from './ManualHelmDialog';
import { AgentWaitingDialog } from './AgentWaitingDialog';
import { EnrollmentDialog } from './EnrollmentDialog';

export default {
  title: 'Teleport/Discover/Kube/EnrollEksClusters/Dialogs',
  loaders: [mswLoader],
};

export const EnrollmentDialogStory = () => (
  <MemoryRouter initialEntries={[{ state: { discover: {} } }]}>
    <EnrollmentDialog
      clusterName={'EKS1'}
      status={'enrolling'}
      error={''}
      close={() => {}}
      retry={() => {}}
    />
  </MemoryRouter>
);
EnrollmentDialogStory.storyName = 'EnrollmentDialog';

export const AgentWaitingDialogStory = () => (
  <MemoryRouter initialEntries={[{ state: { discover: {} } }]}>
    <ContextProvider ctx={createTeleportContext()}>
      <PingTeleportProvider
        interval={100000}
        resourceKind={ResourceKind.Kubernetes}
      >
        <AgentWaitingDialog
          joinResourceId="resource-id"
          status={'awaitingAgent'}
          clusterName={'EKS1'}
          updateWaitingResult={() => {}}
          cancel={() => {}}
          next={() => {}}
        />
      </PingTeleportProvider>
    </ContextProvider>
  </MemoryRouter>
);
AgentWaitingDialogStory.storyName = 'AgentWaitingDialog';
AgentWaitingDialogStory.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
        return res(ctx.delay('infinite'));
      }),
    ],
  },
};

export const AgentWaitingDialogSuccess = () => (
  <MemoryRouter initialEntries={[{ state: { discover: {} } }]}>
    <ContextProvider ctx={createTeleportContext()}>
      <PingTeleportProvider
        interval={100000}
        resourceKind={ResourceKind.Kubernetes}
      >
        <AgentWaitingDialog
          joinResourceId="resource-id"
          status={'success'}
          clusterName={'EKS1'}
          updateWaitingResult={() => {}}
          cancel={() => {}}
          next={() => {}}
        />
      </PingTeleportProvider>
    </ContextProvider>
  </MemoryRouter>
);
AgentWaitingDialogSuccess.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.kubernetesPath, (req, res, ctx) => {
        return res(ctx.delay('infinite'));
      }),
    ],
  },
};

const helmCommand = generateCmd({
  namespace: 'teleport-agent',
  clusterName: 'EKS1',
  proxyAddr: 'teleport-proxy.example.com:1234',
  tokenId: 'token-id',
  clusterVersion: '14.3.2',
  resourceId: 'resource-id',
  isEnterprise: false,
  isCloud: false,
  automaticUpgradesEnabled: false,
  automaticUpgradesTargetVersion: '',
  joinLabels: [
    { name: 'teleport.dev/cloud', value: 'AWS' },
    { name: 'region', value: 'us-east-1' },
    { name: 'account-id', value: '1234567789012' },
  ],
});

export const ManualHelmDialogStory = () => (
  <MemoryRouter initialEntries={[{ state: { discover: {} } }]}>
    <ManualHelmDialog
      command={helmCommand}
      confirmedCommands={() => {}}
      cancel={() => {}}
    />
  </MemoryRouter>
);
ManualHelmDialogStory.storyName = 'ManualHelmDialog';
