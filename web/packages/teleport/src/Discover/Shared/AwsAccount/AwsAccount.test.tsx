/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import {
  IntegrationKind,
  IntegrationStatusCode,
  integrationService,
} from 'teleport/services/integrations';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';
import cfg from 'teleport/config';
import TeleportContext from 'teleport/teleportContext';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';

import {
  DiscoverEventResource,
  userEventService,
} from 'teleport/services/userEvent';
import ResourceService from 'teleport/services/resources';
import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import { ResourceSpec } from 'teleport/Discover/SelectResource';

import { ResourceKind } from '../ResourceKind';

import { AwsAccount } from './AwsAccount';

beforeEach(() => {
  jest.spyOn(integrationService, 'fetchIntegrations').mockResolvedValue({
    items: [
      {
        resourceType: 'integration',
        name: 'aws-oidc-1',
        kind: IntegrationKind.AwsOidc,
        spec: {
          roleArn: 'arn:aws:iam::123456789012:role/test1',
          issuerS3Bucket: '',
          issuerS3Prefix: '',
        },
        statusCode: IntegrationStatusCode.Running,
      },
    ],
  });

  jest
    .spyOn(ResourceService.prototype, 'fetchUnifiedResources')
    .mockResolvedValue({
      agents: [app],
    });
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('non application resource kind', async () => {
  const { ctx, discoverCtx } = getMockedContexts({
    kind: ResourceKind.Server,
    name: '',
    icon: undefined,
    keywords: '',
    event: DiscoverEventResource.Server,
  });

  renderAwsAccount(ctx, discoverCtx);
  await screen.findByText(/aws Integrations/i);

  expect(
    ResourceService.prototype.fetchUnifiedResources
  ).not.toHaveBeenCalled();
  expect(integrationService.fetchIntegrations).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: /next/i })).toBeEnabled();
});

test('with application resource kind for aws console', async () => {
  const { ctx, discoverCtx } = getMockedContexts({
    kind: ResourceKind.Application,
    appMeta: { awsConsole: true },
    name: '',
    icon: undefined,
    keywords: '',
    event: DiscoverEventResource.ApplicationHttp,
  });

  renderAwsAccount(ctx, discoverCtx);
  await screen.findByText(/aws Integrations/i);

  expect(ResourceService.prototype.fetchUnifiedResources).toHaveBeenCalledTimes(
    1
  );
  expect(integrationService.fetchIntegrations).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: /next/i })).toBeEnabled();
});

test('missing permissions for integrations', async () => {
  const { ctx, discoverCtx } = getMockedContexts({
    kind: ResourceKind.Application,
    appMeta: { awsConsole: true },
    name: '',
    icon: undefined,
    keywords: '',
    event: DiscoverEventResource.ApplicationHttp,
  });

  ctx.storeUser.state.acl = getAcl({ noAccess: true });

  renderAwsAccount(ctx, discoverCtx);

  expect(
    screen.getByText(/required permissions for integrating/i)
  ).toBeInTheDocument();
  expect(screen.queryByText(/aws integrations/i)).not.toBeInTheDocument();

  expect(
    ResourceService.prototype.fetchUnifiedResources
  ).not.toHaveBeenCalled();
  expect(integrationService.fetchIntegrations).not.toHaveBeenCalled();

  expect(
    screen.queryByRole('button', { name: /next/i })
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: /back/i })).toBeInTheDocument();
});

function getMockedContexts(resourceSpec: ResourceSpec) {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {},
    currentStep: 0,
    nextStep: jest.fn(),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: resourceSpec,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: jest.fn(),
    eventState: null,
  };

  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);

  return { ctx, discoverCtx };
}

function renderAwsAccount(
  ctx: TeleportContext,
  discoverCtx: DiscoverContextState
) {
  return render(
    <MemoryRouter
      initialEntries={[
        { pathname: cfg.routes.discover, state: { entity: 'application' } },
      ]}
    >
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <AwsAccount />
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
