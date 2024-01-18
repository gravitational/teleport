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
          joinToken={{
            id: 'token-id',
            expiry: null,
            expiryText: '',
            internalResourceId: 'resource-id',
            suggestedLabels: [],
          }}
          status={'awaitingAgent'}
          clusterName={'EKS1'}
          setWaitingResult={() => {}}
          close={() => {}}
        />
      </PingTeleportProvider>
    </ContextProvider>
  </MemoryRouter>
);
// cfg.api.kubernetesPath
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
          joinToken={{
            id: 'token-id',
            expiry: null,
            expiryText: '',
            internalResourceId: 'resource-id',
            suggestedLabels: [],
          }}
          status={'success'}
          clusterName={'EKS1'}
          setWaitingResult={() => {}}
          close={() => {}}
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

const helmCommand = `cat << EOF > prod-cluster-values.yaml
roles: kube,app,discovery
authToken: token-id
proxyAddr: some-long-cluster-public-url-name.cloud.teleport.gravitational.io:1234
kubeClusterName: EKS1
labels:
    teleport.internal/resource-id: resource-id
    teleport.dev/cloud: AWS
    region: us-east-1
    account-id: 1234567789012
EOF
 
helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml \\
--version 14.3.2  --create-namespace --namespace teleport-agent`;
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
