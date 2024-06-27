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
import { render, screen, userEvent } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import {
  IntegrationKind,
  IntegrationStatusCode,
  integrationService,
} from 'teleport/services/integrations';
import { createTeleportContext } from 'teleport/mocks/contexts';
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
import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import { ResourceKind } from 'teleport/Discover/Shared';

import { CreateAppAccess } from './CreateAppAccess';

beforeEach(() => {
  jest.spyOn(integrationService, 'createAwsAppAccess').mockResolvedValue(app);
  jest
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('create app access', async () => {
  const { ctx, discoverCtx } = getMockedContexts();

  renderCreateAppAccess(ctx, discoverCtx);
  await screen.findByText(/bash/i);

  userEvent.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/aws-console/i);
  expect(integrationService.createAwsAppAccess).toHaveBeenCalledTimes(1);
});

function getMockedContexts() {
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
    nextStep: jest.fn(),
    prevStep: () => null,
    onSelectResource: () => null,
    resourceSpec: {
      kind: ResourceKind.Application,
      appMeta: { awsConsole: true },
      name: '',
      icon: undefined,
      keywords: '',
      event: DiscoverEventResource.ApplicationHttp,
    },
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

function renderCreateAppAccess(
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
            <CreateAppAccess />
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
}
