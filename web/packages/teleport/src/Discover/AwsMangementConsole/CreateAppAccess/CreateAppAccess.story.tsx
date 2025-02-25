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

import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import { Info } from 'design/Alert';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
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
import { DiscoverEventResource } from 'teleport/services/userEvent';

import { CreateAppAccess } from './CreateAppAccess';

export default {
  title: 'Teleport/Discover/Application/AwsConsole/CreateApp',
};

export const Success = () => <Component />;
Success.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsAppAccess.createV2, () =>
        HttpResponse.json({ name: 'app-1' })
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
      http.post(
        cfg.api.awsAppAccess.createV2,
        async () => await delay('infinite')
      ),
    ],
  },
};

export const Failed = () => <Component />;
Failed.parameters = {
  msw: {
    handlers: [
      http.post(cfg.api.awsAppAccess.createV2, () =>
        HttpResponse.json(
          {
            message: 'Some kind of error message',
          },
          { status: 403 }
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
      keywords: [],
      event: DiscoverEventResource.ApplicationHttp,
      appMeta: {
        awsConsole: true,
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
          <CreateAppAccess />
        </DiscoverProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};
