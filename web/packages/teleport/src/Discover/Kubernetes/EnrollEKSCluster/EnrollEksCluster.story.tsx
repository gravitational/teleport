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
import React, { useEffect } from 'react';

import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { rest } from 'msw';
import { initialize, mswLoader } from 'msw-storybook-addon';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import {
  AwsEksCluster,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext, getUserContext } from 'teleport/mocks/contexts';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceKind } from 'teleport/Discover/Shared';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';
import { DiscoverEventResource } from 'teleport/services/userEvent/types';
import { Kube } from 'teleport/services/kube';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';

import { EnrollEksCluster } from './EnrollEksCluster';

const { worker } = window.msw;

const integrationName = 'test-oidc';

initialize();
const defaultIsCloud = cfg.isCloud;
const defaultAutomaticUpgrades = cfg.automaticUpgrades;
const defaultAutomaticUpgradesTargetVersion =
  cfg.automaticUpgradesTargetVersion;

export default {
  title: 'Teleport/Discover/Kube/EnrollEksClusters',
  loaders: [mswLoader],
  decorators: [
    Story => {
      worker.resetHandlers();
      clearCachedJoinTokenResult([ResourceKind.Kubernetes]);

      useEffect(() => {
        // Clean up
        return () => {
          cfg.isCloud = defaultIsCloud;
          cfg.automaticUpgrades = defaultAutomaticUpgrades;
          cfg.automaticUpgradesTargetVersion =
            defaultAutomaticUpgradesTargetVersion;
        };
      }, []);
      return <Story />;
    },
  ],
};

const tokenHandler = rest.post(cfg.api.joinTokenPath, (req, res, ctx) => {
  return res(
    ctx.json({
      id: 'token-id',
      suggestedLabels: [
        { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'resource-id' },
      ],
    })
  );
});

const successEnrollmentHandler = rest.post(
  cfg.getEnrollEksClusterUrl(integrationName),
  (req, res, ctx) => {
    return res(
      ctx.delay(1000),
      ctx.status(200),
      ctx.json({
        results: [{ clusterName: 'EKS1' }, { clusterName: 'EKS3' }],
      })
    );
  }
);

const discoveryConfigHandler = rest.post(
  cfg.api.discoveryConfigPath,
  (req, res, ctx) => res(ctx.json({}))
);

export const ClustersList = () => <Component />;

ClustersList.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      successEnrollmentHandler,
      discoveryConfigHandler,
      rest.post(cfg.getListEKSClustersUrl(integrationName), (req, res, ctx) => {
        {
          return res(ctx.json({ clusters: eksClusters }));
        }
      }),
      rest.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        (req, res, ctx) => {
          return res(ctx.json({ items: kubeServers }));
        }
      ),
    ],
  },
};

export const ClustersListInCloud = () => {
  cfg.isCloud = true;
  cfg.automaticUpgrades = true;
  cfg.automaticUpgradesTargetVersion = 'v14.3.2';
  return <Component />;
};

ClustersListInCloud.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      successEnrollmentHandler,
      discoveryConfigHandler,
      rest.post(cfg.getListEKSClustersUrl(integrationName), (req, res, ctx) => {
        {
          return res(ctx.json({ clusters: eksClusters }));
        }
      }),
      rest.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        (req, res, ctx) => {
          return res(ctx.json({ items: kubeServers }));
        }
      ),
    ],
  },
};

export const WithAwsPermissionsError = () => <Component />;

WithAwsPermissionsError.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      rest.post(cfg.getListEKSClustersUrl(integrationName), (req, res, ctx) =>
        res(
          ctx.status(403),
          ctx.json({ message: 'StatusCode: 403, RequestID: operation error' })
        )
      ),
    ],
  },
};

export const WithEnrollmentError = () => <Component />;

WithEnrollmentError.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      rest.post(cfg.getListEKSClustersUrl(integrationName), (req, res, ctx) => {
        {
          return res(ctx.json({ clusters: eksClusters }));
        }
      }),
      rest.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        (req, res, ctx) => {
          return res(ctx.json({ items: kubeServers }));
        }
      ),
      rest.post(
        cfg.getEnrollEksClusterUrl(integrationName),
        (req, res, ctx) => {
          return res(
            ctx.delay(1000),
            ctx.status(200),
            ctx.json({
              results: [
                { clusterName: 'EKS1', error: 'something bad happened' },
                { clusterName: 'EKS3', error: 'something bad happened' },
              ],
            })
          );
        }
      ),
    ],
  },
};

export const WithOtherError = () => <Component />;

WithOtherError.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      rest.post(cfg.getListEKSClustersUrl(integrationName), (req, res, ctx) =>
        res(ctx.status(503))
      ),
    ],
  },
};

const Component = () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'db-name',
      agentMatcherLabels: [],
      kube: {
        kind: 'kube_cluster',
        name: '',
        labels: [],
      },
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: integrationName,
        resourceType: 'integration',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      name: 'Eks',
      kind: ResourceKind.Kubernetes,
      icon: 'Eks',
      keywords: '',
      event: DiscoverEventResource.KubernetesEks,
    },
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
  };

  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'eks' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <PingTeleportProvider
          interval={1000}
          resourceKind={ResourceKind.Kubernetes}
        >
          <DiscoverProvider mockCtx={discoverCtx}>
            <Info>Devs: Select any region to see story state</Info>
            <EnrollEksCluster
              nextStep={discoverCtx.nextStep}
              updateAgentMeta={discoverCtx.updateAgentMeta}
            />
          </DiscoverProvider>
        </PingTeleportProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

const kubeServers: Kube[] = [
  {
    kind: 'kube_cluster',
    name: 'EKS2',
    labels: [
      { name: 'region', value: 'us-east-1' },
      { name: 'account-id', value: '123456789012' },
    ],
  },
];

const eksClusters: AwsEksCluster[] = [
  {
    name: 'EKS1',
    region: 'us-east-1',
    accountId: '123456789012',
    status: 'active',
    labels: [{ name: 'env', value: 'staging' }],
    joinLabels: [
      { name: 'teleport.dev/cloud', value: 'AWS' },
      { name: 'region', value: 'us-east-1' },
      { name: 'account-id', value: '1234567789012' },
    ],
  },
  {
    name: 'EKS2',
    region: 'us-east-1',
    accountId: '123456789012',
    status: 'active',
    labels: [{ name: 'env', value: 'dev' }],
    joinLabels: [
      { name: 'teleport.dev/cloud', value: 'AWS' },
      { name: 'region', value: 'us-east1' },
      { name: 'account-id', value: '1234567789012' },
    ],
  },
  {
    name: 'EKS3',
    region: 'us-east-1',
    accountId: '123456789012',
    status: 'active',
    labels: [{ name: 'env', value: 'prod' }],
    joinLabels: [
      { name: 'teleport.dev/cloud', value: 'AWS' },
      { name: 'region', value: 'us-east-1' },
      { name: 'account-id', value: '1234567789012' },
    ],
  },
];
