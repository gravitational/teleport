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
import { delay, http, HttpResponse } from 'msw';
import { useEffect } from 'react';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ResourceKind } from 'teleport/Discover/Shared';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { clearCachedJoinTokenResult } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext, getUserContext } from 'teleport/mocks/contexts';
import {
  AwsEksCluster,
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { INTERNAL_RESOURCE_ID_LABEL_KEY } from 'teleport/services/joinToken';
import { Kube } from 'teleport/services/kube';
import { DiscoverEventResource } from 'teleport/services/userEvent/types';

import { EnrollEksCluster } from './EnrollEksCluster';

const integrationName = 'test-oidc';

const defaultIsCloud = cfg.isCloud;
const defaultAutomaticUpgrades = cfg.automaticUpgrades;
const defaultAutomaticUpgradesTargetVersion =
  cfg.automaticUpgradesTargetVersion;

export default {
  title: 'Teleport/Discover/Kube/EnrollEksClusters',
  decorators: [
    Story => {
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

const tokenHandler = http.post(cfg.api.discoveryJoinToken.createV2, () => {
  return HttpResponse.json({
    id: 'token-id',
    suggestedLabels: [
      { name: INTERNAL_RESOURCE_ID_LABEL_KEY, value: 'resource-id' },
    ],
  });
});

const successEnrollmentHandler = http.post(
  cfg.getEnrollEksClusterUrl(integrationName),
  async () => {
    await delay(1000);
    return HttpResponse.json(
      {
        results: [{ clusterName: 'EKS1' }, { clusterName: 'EKS3' }],
      },
      { status: 200 }
    );
  }
);

const discoveryConfigHandler = http.post(cfg.api.discoveryConfigPath, () =>
  HttpResponse.json({})
);

export const ClustersList = () => <Component />;

ClustersList.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      successEnrollmentHandler,
      discoveryConfigHandler,
      http.post(cfg.getListEKSClustersUrl(integrationName), () => {
        {
          return HttpResponse.json({ clusters: eksClusters });
        }
      }),
      http.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        () => {
          return HttpResponse.json({ items: kubeServers });
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
      http.post(cfg.getListEKSClustersUrl(integrationName), () => {
        {
          return HttpResponse.json({ clusters: eksClusters });
        }
      }),
      http.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        () => {
          return HttpResponse.json({ items: kubeServers });
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
      http.post(cfg.getListEKSClustersUrl(integrationName), () =>
        HttpResponse.json(
          { message: 'StatusCode: 403, RequestID: operation error' },
          { status: 403 }
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
      http.post(cfg.getListEKSClustersUrl(integrationName), () => {
        {
          return HttpResponse.json({ clusters: eksClusters });
        }
      }),
      http.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        () => {
          return HttpResponse.json({ items: kubeServers });
        }
      ),
      http.post(cfg.getEnrollEksClusterUrl(integrationName), async () => {
        await delay(1000);
        return HttpResponse.json({
          results: [
            { clusterName: 'EKS1', error: 'something bad happened' },
            { clusterName: 'EKS3', error: 'something bad happened' },
          ],
        });
      }),
    ],
  },
};

export const WithAlreadyExistsError = () => (
  <Component devInfoText="select any region, select EKS1 to see already exist error" />
);
WithAlreadyExistsError.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      http.post(cfg.getListEKSClustersUrl(integrationName), () => {
        {
          return HttpResponse.json({ clusters: eksClusters });
        }
      }),
      http.get(
        cfg.getKubernetesUrl(getUserContext().cluster.clusterId, {}),
        () => {
          return HttpResponse.json({ items: kubeServers });
        }
      ),
      http.post(cfg.getEnrollEksClusterUrl(integrationName), async () => {
        await delay(1000);
        return HttpResponse.json({
          results: [
            {
              clusterName: 'EKS1',
              error: 'teleport-kube-agent is already installed on the cluster',
            },
          ],
        });
      }),
    ],
  },
};

export const WithOtherError = () => <Component />;

WithOtherError.parameters = {
  msw: {
    handlers: [
      tokenHandler,
      http.post(cfg.getListEKSClustersUrl(integrationName), () =>
        HttpResponse.json(
          {
            error: { message: 'Whoops, something went wrong.' },
          },
          { status: 503 }
        )
      ),
    ],
  },
};

const Component = ({ devInfoText = '' }) => {
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
      icon: 'eks',
      keywords: [],
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
            <Info>
              {devInfoText || 'Devs: Select any region to see story state'}
            </Info>
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
    authenticationMode: 'API',
    endpointPublicAccess: true,
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
    authenticationMode: 'API',
    endpointPublicAccess: true,
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
    authenticationMode: 'API',
    endpointPublicAccess: true,
  },
  {
    name: 'EKS4',
    region: 'us-east-1',
    accountId: '123456789012',
    status: 'active',
    labels: [{ name: 'env', value: 'prod' }],
    joinLabels: [
      { name: 'teleport.dev/cloud', value: 'AWS' },
      { name: 'region', value: 'us-east-1' },
      { name: 'account-id', value: '1234567789012' },
    ],
    authenticationMode: 'CONFIG_MAP',
    endpointPublicAccess: true,
  },
  {
    name: 'EKS5',
    region: 'us-east-1',
    accountId: '123456789012',
    status: 'active',
    labels: [{ name: 'env', value: 'prod' }],
    joinLabels: [
      { name: 'teleport.dev/cloud', value: 'AWS' },
      { name: 'region', value: 'us-east-1' },
      { name: 'account-id', value: '1234567789012' },
    ],
    authenticationMode: 'API_AND_CONFIG_MAP',
    endpointPublicAccess: false,
  },
];
