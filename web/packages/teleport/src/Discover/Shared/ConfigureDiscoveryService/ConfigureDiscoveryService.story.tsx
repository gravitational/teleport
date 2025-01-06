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

import { http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ServerLocation } from 'teleport/Discover/SelectResource';
import { ResourceKind } from 'teleport/Discover/Shared';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import {
  DiscoverDiscoveryConfigMethod,
  DiscoverEventResource,
} from 'teleport/services/userEvent';

import { ConfigureDiscoveryService as Comp } from './ConfigureDiscoveryService';

export default {
  title: 'Teleport/Discover/Shared/ConfigureDiscoveryService',
};

export const Server = () => {
  return <Component kind={ResourceKind.Server} />;
};

export const Database = () => {
  return <Component kind={ResourceKind.Database} />;
};

export const WithCreateConfig = () => {
  return <Component kind={ResourceKind.Database} withCreateConfig={true} />;
};
WithCreateConfig.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
    ],
  },
};

export const WithCreateConfigFailed = () => {
  return <Component kind={ResourceKind.Database} withCreateConfig={true} />;
};
WithCreateConfigFailed.parameters = {
  msw: {
    handlers: [
      http.post(
        cfg.api.discoveryConfigPath,
        () =>
          HttpResponse.json(
            {
              message: 'Whoops, creating config error',
            },
            { status: 403 }
          ),
        { once: true }
      ),
      http.post(cfg.api.discoveryConfigPath, () => HttpResponse.json({})),
    ],
  },
};

const Component = ({
  kind,
  withCreateConfig = false,
}: {
  kind: ResourceKind;
  withCreateConfig?: boolean;
}) => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'aws-console',
      agentMatcherLabels: [],
      awsRegion: 'ap-south-1',
      awsIntegration: {
        kind: IntegrationKind.AwsOidc,
        name: 'some-oidc-name',
        resourceType: 'integration',
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
      autoDiscovery: {
        config: {
          name: 'discovery-config-name',
          discoveryGroup: 'discovery-group-name',
          aws: [],
        },
      },
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      name: '',
      kind,
      icon: null,
      keywords: [],
      event: DiscoverEventResource.Ec2Instance,
      nodeMeta: {
        location: ServerLocation.Aws,
        discoveryConfigMethod: DiscoverDiscoveryConfigMethod.AwsEc2Ssm,
      },
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

  cfg.proxyCluster = 'localhost';
  return (
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: '' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          {withCreateConfig && (
            <Info>Devs: Click next to see create config dialog</Info>
          )}
          <Comp withCreateConfig={withCreateConfig} />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
