/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { QueryClientProvider } from '@tanstack/react-query';
import selectEvent from 'react-select-event';

import { copyToClipboard } from 'design/utils/copyToClipboard';
import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
  within,
} from 'design/utils/testing';
import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from 'teleport';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  IntegrationWithSummary,
  integrationService,
} from 'teleport/services/integrations';

import { SettingsTab } from './SettingsTab';

jest.mock('design/utils/copyToClipboard', () => ({
  copyToClipboard: jest.fn(),
}));

const baseStats: IntegrationWithSummary = {
  name: 'my-aws-integration',
  subKind: IntegrationKind.AwsOidc,
  unresolvedUserTasks: 0,
  userTasks: [],
  awsra: {} as any,
  awsoidc: {} as any,
  awsec2: {} as any,
  awsrds: {} as any,
  awseks: {} as any,
  azurevm: {} as any,
  rolesAnywhereProfileSync: {} as any,
};

const baseIntegration: IntegrationAwsOidc = {
  resourceType: 'integration',
  kind: IntegrationKind.AwsOidc,
  name: 'my-aws-integration',
  spec: {
    roleArn: 'arn:aws:iam::123456789012:role/example',
  },
  statusCode: 1,
};

function setupMocks(integration: IntegrationAwsOidc = baseIntegration) {
  jest
    .spyOn(integrationService, 'fetchIntegration')
    .mockResolvedValue(integration);
  jest.spyOn(integrationService, 'fetchIntegrationRules').mockResolvedValue({
    rules: [],
    nextKey: '',
  });
}

function mockRdsRule() {
  jest
    .mocked(integrationService.fetchIntegrationRules)
    .mockImplementation((_name, resourceType) =>
      Promise.resolve({
        rules:
          resourceType === AwsResource.rds
            ? [
                {
                  resourceType: AwsResource.rds,
                  region: 'us-east-1',
                  labelMatcher: [],
                  discoveryConfig: 'my-discovery-config',
                  lastSync: 0,
                },
              ]
            : [],
        nextKey: '',
      })
    );
}

function renderSettingsTab() {
  const ctx = createTeleportContext();
  ctx.storeUser.state.cluster.authVersion = '1.0.0';

  return render(
    <ContextProvider ctx={ctx}>
      <QueryClientProvider client={testQueryClient}>
        <SettingsTab
          stats={baseStats}
          header={<div>my-aws-integration: Edit configuration</div>}
        />
      </QueryClientProvider>
    </ContextProvider>
  );
}

describe('SettingsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    testQueryClient.clear();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('organization scope', () => {
    test('does not render for single account', async () => {
      setupMocks();
      renderSettingsTab();

      expect(
        screen.getByText('my-aws-integration: Edit configuration')
      ).toBeInTheDocument();

      await waitFor(() => {
        expect(
          screen.getByRole('radio', { name: 'Single Account' })
        ).toBeChecked();
      });
      expect(
        screen.queryByText(/Include Organizational Units/i)
      ).not.toBeInTheDocument();
    });

    test('renders with organization scope', async () => {
      setupMocks({
        ...baseIntegration,
        spec: {
          ...baseIntegration.spec,
          organization: {
            includeUnits: ['ou-1', 'ou-2'],
            excludeUnits: ['ou-3'],
          },
        },
      });
      renderSettingsTab();

      await waitFor(() => {
        expect(
          screen.getByRole('radio', { name: 'Organization' })
        ).toBeChecked();
      });

      const includeGroup = screen.getByRole('group', {
        name: /Include Organizational Units/i,
      });
      expect(
        within(includeGroup).getByDisplayValue('ou-1')
      ).toBeInTheDocument();
      expect(
        within(includeGroup).getByDisplayValue('ou-2')
      ).toBeInTheDocument();

      const excludeGroup = screen.getByRole('group', {
        name: /Exclude Organizational Units/i,
      });
      expect(
        within(excludeGroup).getByDisplayValue('ou-3')
      ).toBeInTheDocument();
    });
  });

  test('rendering warning dialog after copying Terraform while editing', async () => {
    setupMocks();
    mockRdsRule();
    renderSettingsTab();

    await userEvent.click(
      await screen.findByRole('button', { name: 'Copy Terraform Module' })
    );

    const dialog = screen.getByRole('dialog');
    expect(
      within(dialog).getByText('Terraform Module Copied')
    ).toBeInTheDocument();
    expect(copyToClipboard).toHaveBeenCalledTimes(1);

    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Got It' })
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(copyToClipboard).toHaveBeenCalledTimes(1);

    // Render warning when copying directly from the editor.
    await userEvent.click(
      screen.getByRole('button', { name: 'Terraform Configuration' })
    );
    fireEvent.copy(screen.getByTestId('mock-text-editor'));
    expect(screen.getByText('Terraform Module Copied')).toBeInTheDocument();
  });

  test('warning dialog is not rendered when deploying a new database service', async () => {
    setupMocks();
    mockRdsRule();
    renderSettingsTab();

    await userEvent.click(
      await screen.findByRole('radio', {
        name: /Deploy a new database service for me/i,
      })
    );
    await selectEvent.select(
      screen.getByRole('combobox', { name: 'Region' }),
      'US East (N. Virginia)'
    );
    fireEvent.change(screen.getByPlaceholderText('vpc-0123456789abcdef0'), {
      target: { value: 'vpc-123' },
    });
    const securityGroupInput = screen.getByRole('combobox', {
      name: 'Security group IDs',
    });
    fireEvent.change(securityGroupInput, {
      target: { value: 'sg-123' },
    });
    fireEvent.keyDown(securityGroupInput, { key: 'Enter', code: 'Enter' });

    await userEvent.click(
      screen.getByRole('button', { name: 'Copy Terraform Module' })
    );

    expect(copyToClipboard).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  test('warning dialog is not rendered if RDS is not configured', async () => {
    setupMocks();
    renderSettingsTab();

    await userEvent.click(
      await screen.findByRole('checkbox', { name: 'EC2 Instances' })
    );
    await userEvent.click(
      screen.getByRole('button', { name: 'Copy Terraform Module' })
    );

    expect(copyToClipboard).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  test('defaulting to deploying a database service when enabling RDS', async () => {
    setupMocks();
    renderSettingsTab();

    await userEvent.click(
      await screen.findByRole('checkbox', { name: 'RDS Databases' })
    );

    expect(
      screen.getByRole('radio', {
        name: /Deploy a new database service for me/i,
      })
    ).toBeChecked();
    expect(
      screen.getByRole('combobox', { name: 'Region' })
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('vpc-0123456789abcdef0')
    ).toBeInTheDocument();
    expect(
      screen.getByRole('combobox', { name: 'Security group IDs' })
    ).toBeInTheDocument();
  });
});
