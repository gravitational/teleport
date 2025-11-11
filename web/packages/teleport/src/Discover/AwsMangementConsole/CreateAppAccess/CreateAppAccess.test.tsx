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

import { render, screen, userEvent } from 'design/utils/testing';

import { app } from 'teleport/Discover/AwsMangementConsole/fixtures';
import {
  RequiredDiscoverProviders,
  resourceSpecAppAwsCliConsole,
} from 'teleport/Discover/Fixtures/fixtures';
import { AgentMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  integrationService,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import { ProxyRequiresUpgrade } from 'teleport/services/version/unsupported';

import { CreateAppAccess } from './CreateAppAccess';

beforeEach(() => {
  vi
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);
  vi.spyOn(integrationService, 'createAwsAppAccessV2').mockResolvedValue(app);
  vi
    .spyOn(userEventService, 'captureDiscoverEvent')
    .mockResolvedValue(undefined as never);
});

afterEach(() => {
  vi.restoreAllMocks();
});

test('create app access', async () => {
  vi.spyOn(integrationService, 'createAwsAppAccess').mockResolvedValue(app);

  renderCreateAppAccess();
  await screen.findByText(/bash/i);

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/aws-console/i);
  expect(integrationService.createAwsAppAccessV2).toHaveBeenCalledTimes(1);
  expect(integrationService.createAwsAppAccess).not.toHaveBeenCalled();
});

test('create app access with v1 endpoint auto retry', async () => {
  vi
    .spyOn(integrationService, 'createAwsAppAccessV2')
    .mockRejectedValueOnce(new Error(ProxyRequiresUpgrade));
  vi.spyOn(integrationService, 'createAwsAppAccess').mockResolvedValue(app);

  renderCreateAppAccess();
  await screen.findByText(/bash/i);

  await userEvent.click(screen.getByRole('button', { name: /next/i }));
  await screen.findByText(/aws-console/i);

  expect(integrationService.createAwsAppAccessV2).toHaveBeenCalledTimes(1);
  expect(integrationService.createAwsAppAccess).toHaveBeenCalledTimes(1);
});

const agentMeta: AgentMeta = {
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
};

function renderCreateAppAccess() {
  return render(
    <RequiredDiscoverProviders
      agentMeta={agentMeta}
      resourceSpec={resourceSpecAppAwsCliConsole}
    >
      <CreateAppAccess />
    </RequiredDiscoverProviders>
  );
}
