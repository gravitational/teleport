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

import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import { ResourceSpec } from 'teleport/Discover/SelectResource';
import {
  DiscoverContextState,
  DiscoverProvider,
} from 'teleport/Discover/useDiscover';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import ResourceService from 'teleport/services/resources';
import {
  DiscoverEventResource,
  userEventService,
} from 'teleport/services/userEvent';
import TeleportContext from 'teleport/teleportContext';

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
    keywords: [],
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
    keywords: [],
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
    keywords: [],
    event: DiscoverEventResource.ApplicationHttp,
  });

  ctx.storeUser.state.acl = getAcl({ noAccess: true });

  renderAwsAccount(ctx, discoverCtx);

  expect(
    screen.getByText(/permissions required to set up this integration/i)
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

test('health check is called after selecting an aws integration', async () => {
  const { ctx, discoverCtx, spyPing } = getMockedContexts({
    kind: ResourceKind.Application,
    appMeta: { awsConsole: true },
    name: '',
    icon: undefined,
    keywords: [],
    event: DiscoverEventResource.ApplicationHttp,
  });

  renderAwsAccount(ctx, discoverCtx);

  await screen.findByText(/AWS Integrations/i);

  const selectContainer = screen.getByRole('combobox');
  fireEvent.mouseDown(selectContainer);
  fireEvent.keyPress(selectContainer, { key: 'Enter' });

  expect(spyPing).toHaveBeenCalledTimes(1);
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

  const spyPing = jest
    .spyOn(integrationService, 'fetchIntegrations')
    .mockResolvedValue({
      items: [
        {
          resourceType: 'integration',
          name: 'aws-oidc-1',
          kind: IntegrationKind.AwsOidc,
          spec: { roleArn: '111' },
          statusCode: IntegrationStatusCode.Running,
        },
      ],
    });

  return { ctx, discoverCtx, spyPing };
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
