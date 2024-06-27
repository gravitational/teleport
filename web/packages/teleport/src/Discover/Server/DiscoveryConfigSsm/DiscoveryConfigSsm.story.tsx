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

import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';
import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  DiscoverProvider,
  DiscoverContextState,
} from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { ResourceKind } from 'teleport/Discover/Shared';
import {
  DiscoverDiscoveryConfigMethod,
  DiscoverEventResource,
} from 'teleport/services/userEvent';
import { ServerLocation } from 'teleport/Discover/SelectResource';

import { DiscoveryConfigSsm } from './DiscoveryConfigSsm';

initialize();

const defaultIsCloud = cfg.isCloud;
export default {
  title: 'Teleport/Discover/Server/EC2/DiscoveryConfigSsm',
  loaders: [mswLoader],
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.isCloud = defaultIsCloud;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const SuccessCloud = () => {
  cfg.isCloud = true;
  return <Component />;
};
SuccessCloud.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.joinTokenPath, (req, res, ctx) =>
        res(ctx.json({ id: 'token-id' }))
      ),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(ctx.json({ name: 'discovery-cfg-name' }))
      ),
    ],
  },
};

export const SuccessSelfHosted = () => <Component />;
SuccessSelfHosted.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.joinTokenPath, (req, res, ctx) =>
        res(ctx.json({ id: 'token-id' }))
      ),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(ctx.json({ name: 'discovery-cfg-name' }))
      ),
    ],
  },
};

export const Loading = () => {
  cfg.isCloud = true;
  return <Component />;
};
Loading.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.joinTokenPath, (req, res, ctx) =>
        res(ctx.json({ id: 'token-id' }))
      ),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(ctx.delay('infinite'))
      ),
    ],
  },
};

export const Failed = () => <Component />;
Failed.parameters = {
  msw: {
    handlers: [
      rest.post(cfg.api.joinTokenPath, (req, res, ctx) =>
        res(ctx.json({ id: 'token-id' }))
      ),
      rest.post(cfg.api.discoveryConfigPath, (req, res, ctx) =>
        res(
          ctx.status(403),
          ctx.json({ message: 'Some kind of error message' })
        )
      ),
    ],
  },
};

const Component = () => {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {
      resourceName: 'aws-console',
      agentMatcherLabels: [],
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
    },
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      name: '',
      kind: ResourceKind.Application,
      icon: null,
      keywords: '',
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
        { pathname: cfg.routes.discover, state: { entity: 'application' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <DiscoverProvider mockCtx={discoverCtx}>
          <Info>Devs: Click next to see next state</Info>
          <DiscoveryConfigSsm />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
