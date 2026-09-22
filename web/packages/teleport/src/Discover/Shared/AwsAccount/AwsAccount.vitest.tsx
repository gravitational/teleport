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

import { afterEach, beforeEach, expect, test, vi } from 'vitest';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import {
  RequiredDiscoverProviders,
  resourceSpecAppAwsCliConsole,
  resourceSpecServerLinuxUbuntu,
} from 'teleport/Discover/Fixtures/fixtures';
import type { SelectResourceSpec } from 'teleport/Discover/SelectResource/resources';
import type { DiscoverContextState } from 'teleport/Discover/useDiscover';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
  type IntegrationAwsOidc,
} from 'teleport/services/integrations';
import ResourceService from 'teleport/services/resources';
import { userEventService } from 'teleport/services/userEvent';
import type TeleportContext from 'teleport/teleportContext';

import { AwsAccount } from './AwsAccount';

beforeEach(() => {
  vi.spyOn(integrationService, 'fetchIntegrations').mockResolvedValue({
    items: [integration],
  });
  vi.spyOn(integrationService, 'pingAwsOidcIntegration').mockResolvedValue({
    accountId: '123456789012',
    arn: integration.spec.roleArn,
    userId: 'test-user',
  });
  vi.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  ).mockResolvedValue({
    agents: [app],
  });
  vi.spyOn(userEventService, 'captureDiscoverEvent').mockResolvedValue(
    undefined as never
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

test('loads integrations for a server without fetching applications', async () => {
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecServerLinuxUbuntu);

  renderAwsAccount(ctx, discoverCtx);

  expect(await screen.findByText('AWS Integrations')).toBeInTheDocument();
  expect(
    ResourceService.prototype.fetchUnifiedResources
  ).not.toHaveBeenCalled();
  expect(integrationService.fetchIntegrations).toHaveBeenCalledTimes(1);
  expect(integrationService.pingAwsOidcIntegration).not.toHaveBeenCalled();
  expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
});

test('loads applications when enrolling an AWS console application', async () => {
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecAppAwsCliConsole);

  renderAwsAccount(ctx, discoverCtx);

  expect(await screen.findByText('AWS Integrations')).toBeInTheDocument();
  expect(ResourceService.prototype.fetchUnifiedResources).toHaveBeenCalledTimes(
    1
  );
  expect(integrationService.fetchIntegrations).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
});

test('shows missing permissions without fetching or checking integrations', () => {
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecAppAwsCliConsole);
  ctx.storeUser.setState({ acl: getAcl({ noAccess: true }) });

  renderAwsAccount(ctx, discoverCtx);

  expect(screen.queryByText('AWS Integrations')).not.toBeInTheDocument();
  expect(
    ResourceService.prototype.fetchUnifiedResources
  ).not.toHaveBeenCalled();
  expect(integrationService.fetchIntegrations).not.toHaveBeenCalled();
  expect(integrationService.pingAwsOidcIntegration).not.toHaveBeenCalled();
  expect(
    screen.queryByRole('button', { name: 'Next' })
  ).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
});

test('requires a selection when multiple integrations are available', async () => {
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecServerLinuxUbuntu);
  vi.mocked(integrationService.fetchIntegrations).mockResolvedValue({
    items: [integration, { ...integration, name: 'aws-oidc-2' }],
  });

  renderAwsAccount(ctx, discoverCtx);

  expect(await screen.findByText('AWS Integrations')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  expect(integrationService.pingAwsOidcIntegration).not.toHaveBeenCalled();
});

test('offers enrollment when no AWS integrations exist', async () => {
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecServerLinuxUbuntu);
  vi.mocked(integrationService.fetchIntegrations).mockResolvedValue({
    items: [],
  });

  renderAwsAccount(ctx, discoverCtx);

  expect(
    await screen.findByRole('link', { name: 'Set up AWS Account' })
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  expect(integrationService.pingAwsOidcIntegration).not.toHaveBeenCalled();
});

test('checks the selected integration before proceeding', async () => {
  const user = userEvent.setup();
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecServerLinuxUbuntu);
  renderAwsAccount(ctx, discoverCtx);

  await user.click(await screen.findByRole('button', { name: 'Next' }));

  await waitFor(() => expect(discoverCtx.nextStep).toHaveBeenCalledTimes(1));
  expect(
    integrationService.pingAwsOidcIntegration
  ).toHaveBeenCalledExactlyOnceWith(
    {
      clusterId: ctx.storeUser.state.cluster.clusterId,
      integrationName: integration.name,
    },
    { roleArn: '' }
  );
  expect(discoverCtx.updateAgentMeta).toHaveBeenCalledWith({
    awsIntegration: integration,
  });
});

test('stays on the current step when the integration health check fails', async () => {
  const user = userEvent.setup();
  const { ctx, discoverCtx } = getMockedContexts(resourceSpecServerLinuxUbuntu);
  vi.mocked(integrationService.pingAwsOidcIntegration).mockRejectedValue(
    new Error('Access denied')
  );
  renderAwsAccount(ctx, discoverCtx);

  await user.click(await screen.findByRole('button', { name: 'Next' }));

  expect(
    await screen.findByText(
      'Health check failed for the selected AWS integration: Access denied'
    )
  ).toBeInTheDocument();
  expect(discoverCtx.nextStep).not.toHaveBeenCalled();
});

function getMockedContexts(resourceSpec: SelectResourceSpec) {
  const ctx = createTeleportContext();
  const discoverCtx: DiscoverContextState = {
    agentMeta: {},
    currentStep: 0,
    nextStep: vi.fn(),
    prevStep: vi.fn(),
    onSelectResource: vi.fn(),
    resourceSpec,
    exitFlow: vi.fn(),
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: vi.fn(),
    updateAgentMeta: vi.fn(),
    emitErrorEvent: vi.fn(),
    emitEvent: vi.fn(),
    eventState: null,
  };

  return { ctx, discoverCtx };
}

function renderAwsAccount(
  ctx: TeleportContext,
  discoverCtx: DiscoverContextState
) {
  return render(
    <RequiredDiscoverProviders
      agentMeta={discoverCtx.agentMeta}
      resourceSpec={discoverCtx.resourceSpec}
      teleportCtx={ctx}
      discoverCtx={discoverCtx}
    >
      <AwsAccount />
    </RequiredDiscoverProviders>
  );
}

const integration: IntegrationAwsOidc = {
  resourceType: 'integration',
  name: 'aws-oidc-1',
  kind: IntegrationKind.AwsOidc,
  spec: {
    roleArn: 'arn:aws:iam::123456789012:role/test1',
    issuerS3Bucket: '',
    issuerS3Prefix: '',
  },
  statusCode: IntegrationStatusCode.Running,
};
