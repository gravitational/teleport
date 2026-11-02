/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

import { copyToClipboard } from 'design/utils/copyToClipboard';
import { act, fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';
import { integrationService } from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import {
  IntegrationEnrollCodeType,
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent/types';

import { EnrollAws } from './EnrollAws';

jest.mock('design/utils/copyToClipboard', () => ({
  copyToClipboard: jest.fn(),
}));

const defaultProxyCluster = cfg.proxyCluster;

describe('EnrollAws', () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  function renderEnrollAws() {
    const ctx = createTeleportContext();
    ctx.storeUser.state.cluster.authVersion = '1.0.0';

    return render(
      <ContextProvider ctx={ctx}>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <InfoGuidePanelProvider>
              <ContentMinWidth>
                <EnrollAws />
              </ContentMinWidth>
            </InfoGuidePanelProvider>
          </MemoryRouter>
        </QueryClientProvider>
      </ContextProvider>
    );
  }

  beforeEach(() => {
    jest.clearAllMocks();
    queryClient.clear();
    cfg.proxyCluster = 'my-cluster.cloud.gravitational.io';
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
  });

  afterEach(() => {
    jest.restoreAllMocks();
    cfg.proxyCluster = defaultProxyCluster;
  });

  test('emits start event on mount', () => {
    renderEnrollAws();

    expect(userEventService.captureIntegrationEnrollEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: IntegrationEnrollEvent.Started,
        eventData: expect.objectContaining({
          kind: IntegrationEnrollKind.AwsCloud,
        }),
      })
    );
  });

  test('info guide renders by default', async () => {
    renderEnrollAws();

    expect(screen.getByRole('button', { name: 'Info Guide' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );

    await waitFor(() => {
      expect(screen.getByText(/Reference Links/i)).toBeInTheDocument();
    });
  });

  test('copy terraform configuration button validates and copies to clipboard', async () => {
    renderEnrollAws();

    const input = screen.getByLabelText(/integration name/i);

    expect(input).toHaveDisplayValue(/^aws-integration-/);

    fireEvent.change(input, {
      target: { value: '' },
    });

    const copyButtons = screen.getAllByRole('button', {
      name: /copy terraform module/i,
    });
    const copyButton = copyButtons[0];
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(
        screen.getByText(/integration name is required/i)
      ).toBeInTheDocument();
    });
    expect(
      screen.getByText(/at least one security group ID is required/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/region is required/i)).toBeInTheDocument();

    fireEvent.change(input, {
      target: { value: 'test-integration' },
    });
    await enterDatabaseServiceDetails();

    fireEvent.click(copyButton);

    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('teleport_integration_name')
    );

    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('"test-integration"')
    );

    expect(userEventService.captureIntegrationEnrollEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: IntegrationEnrollEvent.CodeCopy,
        eventData: expect.objectContaining({
          kind: IntegrationEnrollKind.AwsCloud,
          codeType: IntegrationEnrollCodeType.Terraform,
        }),
      })
    );
  });

  test('check integration button validates form', async () => {
    renderEnrollAws();

    const input = screen.getByLabelText(/integration name/i);

    // change to invalid name
    fireEvent.change(input, {
      target: { value: '0cool' },
    });

    const checkButton = screen.getByRole('button', {
      name: /check integration/i,
    });
    fireEvent.click(checkButton);

    await waitFor(() => {
      expect(
        screen.getByText(/name must start with an alphabetic/i)
      ).toBeInTheDocument();
    });
  });

  test('queries for integration and shows success when found', async () => {
    jest
      .spyOn(integrationService, 'fetchIntegration')
      .mockResolvedValue({ name: 'test-integration' } as any);

    renderEnrollAws();

    fireEvent.change(screen.getByLabelText(/integration name/i), {
      target: { value: 'test-integration' },
    });
    await enterDatabaseServiceDetails();

    const checkButton = screen.getByRole('button', {
      name: /check integration/i,
    });
    fireEvent.click(checkButton);

    expect(userEventService.captureIntegrationEnrollEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: IntegrationEnrollEvent.Step,
        eventData: expect.objectContaining({
          kind: IntegrationEnrollKind.AwsCloud,
          step: IntegrationEnrollStep.VerifyIntegration,
        }),
      })
    );

    const success = await screen.findByText(/successfully added/i);
    expect(success).toBeInTheDocument();

    expect(userEventService.captureIntegrationEnrollEvent).toHaveBeenCalledWith(
      expect.objectContaining({
        event: IntegrationEnrollEvent.Complete,
        eventData: expect.objectContaining({
          kind: IntegrationEnrollKind.AwsCloud,
        }),
      })
    );

    const viewIntegrationLinks = screen.getAllByRole('link', {
      name: /^view integration$/i,
    });

    viewIntegrationLinks.forEach(link => {
      expect(link).toHaveAttribute(
        'href',
        expect.stringContaining('/test-integration')
      );
    });
  });

  test('integration not found shows an error', async () => {
    jest.useFakeTimers();

    jest.spyOn(integrationService, 'fetchIntegration').mockRejectedValue(
      new ApiError({
        message: '',
        response: { status: 404 } as Response,
      })
    );

    renderEnrollAws();

    fireEvent.change(screen.getByLabelText(/integration name/i), {
      target: { value: 'missing-integration' },
    });
    await enterDatabaseServiceDetails();

    fireEvent.click(
      screen.getByRole('button', {
        name: /check integration/i,
      })
    );

    // wait until polling fails
    await act(async () => {
      await jest.advanceTimersByTimeAsync(35000);
    });

    expect(
      screen.getByRole('button', {
        name: /^view integration$/i,
      })
    ).toBeDisabled();

    expect(
      userEventService.captureIntegrationEnrollEvent
    ).not.toHaveBeenCalledWith(
      expect.objectContaining({
        event: IntegrationEnrollEvent.Complete,
      })
    );

    jest.useRealTimers();
  });

  test('panel switches between info and terraform tabs', async () => {
    renderEnrollAws();

    expect(screen.getByRole('button', { name: 'Info Guide' })).toHaveAttribute(
      'aria-pressed',
      'true'
    );

    await waitFor(() => {
      expect(screen.getByText(/Reference Links/i)).toBeInTheDocument();
    });

    const terraformButton = screen.getByRole('button', {
      name: 'Terraform Configuration',
    });
    fireEvent.click(terraformButton);

    await waitFor(() => {
      expect(screen.getByText(/module "aws_discovery"/)).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.queryByText('Reference Links')).not.toBeInTheDocument();
    });

    const infoButton = screen.getByRole('button', {
      name: 'Info Guide',
    });
    fireEvent.click(infoButton);

    await waitFor(() => {
      expect(screen.getByText(/Reference Links/i)).toBeInTheDocument();
    });
  });

  test('all resource types are enabled by default and are included in the Terraform configuration', async () => {
    renderEnrollAws();
    await enterDatabaseServiceDetails();

    // Default selections:
    expect(
      screen.getByRole('checkbox', { name: 'EC2 Instances' })
    ).toBeChecked();
    expect(
      screen.getByRole('checkbox', { name: 'RDS Databases' })
    ).toBeChecked();
    expect(
      screen.getByRole('checkbox', { name: 'EKS Clusters' })
    ).toBeChecked();
    expect(
      screen.getByRole('radio', {
        name: /Deploy a new database service for me/i,
      })
    ).toBeChecked();
    expect(
      screen.getByRole('checkbox', {
        name: 'Enable Kubernetes App Discovery',
      })
    ).toBeChecked();

    fireEvent.click(
      screen.getAllByRole('button', {
        name: /copy terraform module/i,
      })[0]
    );

    // Default generated terraform config:
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringMatching(/types\s+= \["ec2"\]/)
    );
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringMatching(/types\s+= \["rds"\]/)
    );
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringMatching(/types\s+= \["eks"\]/)
    );

    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('module "db_agent"')
    );
  });

  test('toggling database service module in terraform config', async () => {
    renderEnrollAws();
    await enterDatabaseServiceDetails();

    expect(
      screen.getByRole('combobox', { name: 'RDS Databases regions' })
    ).toBeInTheDocument();
    expect(
      screen.getByRole('combobox', { name: 'Region' })
    ).toBeInTheDocument();

    const copyButton = screen.getAllByRole('button', {
      name: /copy terraform module/i,
    })[0];

    // default enables db service
    fireEvent.click(copyButton);
    expect(copyToClipboard).toHaveBeenLastCalledWith(
      expect.stringContaining('module "db_agent"')
    );

    fireEvent.click(
      screen.getByRole('radio', {
        name: /I already have a Database Service/i,
      })
    );
    expect(
      screen.getByRole('combobox', { name: 'RDS Databases regions' })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: 'Region' })
    ).not.toBeInTheDocument();
    fireEvent.click(copyButton);

    expect(copyToClipboard).toHaveBeenLastCalledWith(
      expect.not.stringContaining('module "db_agent"')
    );
  });

  test('root and nested regions stay in sync and support multiple VPCs per region', async () => {
    renderEnrollAws();

    const rootRegionSelect = screen.getByRole('combobox', {
      name: 'RDS Databases regions',
    });
    await selectEvent.select(rootRegionSelect, 'US East (N. Virginia)');
    fireEvent.keyDown(rootRegionSelect, { key: 'Escape', code: 'Escape' });
    expect(screen.getByText('US East (N. Virginia)')).toBeInTheDocument();

    const vpcInputs = screen.getAllByPlaceholderText('vpc-0123456789abcdef0');
    expect(vpcInputs).toHaveLength(1);
    fireEvent.change(vpcInputs[0], { target: { value: 'vpc-one' } });

    const firstSecurityGroupInput = screen.getByRole('combobox', {
      name: 'Security group IDs',
    });
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();

    fireEvent.change(firstSecurityGroupInput, {
      target: { value: 'sg-one' },
    });
    expect(screen.getByText('Press Enter to add "sg-one"')).toBeInTheDocument();
    fireEvent.keyDown(firstSecurityGroupInput, {
      key: 'Enter',
      code: 'Enter',
    });
    expect(screen.getByText('sg-one')).toBeInTheDocument();

    fireEvent.change(firstSecurityGroupInput, {
      target: { value: 'sg-two' },
    });
    fireEvent.keyDown(firstSecurityGroupInput, {
      key: 'Enter',
      code: 'Enter',
    });
    expect(screen.getByText('sg-two')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Add another VPC/i }));

    const updatedVpcInputs = screen.getAllByPlaceholderText(
      'vpc-0123456789abcdef0'
    );
    expect(updatedVpcInputs).toHaveLength(2);
    fireEvent.change(updatedVpcInputs[1], { target: { value: 'vpc-two' } });

    const securityGroupInputs = screen.getAllByRole('combobox', {
      name: 'Security group IDs',
    });
    expect(securityGroupInputs).toHaveLength(2);
    fireEvent.change(securityGroupInputs[1], {
      target: { value: 'sg-three' },
    });
    fireEvent.keyDown(securityGroupInputs[1], {
      key: 'Enter',
      code: 'Enter',
    });

    fireEvent.click(
      screen.getByRole('button', { name: /Add another region/i })
    );

    const regionInputs = screen.getAllByRole('combobox', { name: 'Region' });
    expect(regionInputs).toHaveLength(2);
    selectEvent.openMenu(regionInputs[1]);

    // Already selected regions are disabled
    expect(
      screen.getByRole('option', { name: /US East \(N\. Virginia\)/ })
    ).toHaveAttribute('aria-disabled', 'true');

    await selectEvent.select(regionInputs[1], 'US West (Oregon)');
    expect(screen.getByText('2 regions selected')).toBeInTheDocument();

    const allVpcInputs = screen.getAllByPlaceholderText(
      'vpc-0123456789abcdef0'
    );
    expect(allVpcInputs).toHaveLength(3);
    fireEvent.change(allVpcInputs[2], { target: { value: 'vpc-three' } });

    const allSecurityGroupInputs = screen.getAllByRole('combobox', {
      name: 'Security group IDs',
    });
    expect(allSecurityGroupInputs).toHaveLength(3);
    fireEvent.change(allSecurityGroupInputs[2], {
      target: { value: 'sg-four' },
    });
    fireEvent.keyDown(allSecurityGroupInputs[2], {
      key: 'Enter',
      code: 'Enter',
    });

    fireEvent.click(
      screen.getAllByRole('button', {
        name: /copy terraform module/i,
      })[0]
    );

    expect(copyToClipboard).toHaveBeenCalledTimes(1);
    const terraformConfig = jest.mocked(copyToClipboard).mock.calls[0][0];

    expect(terraformConfig).toContain('us-east-1');
    expect(terraformConfig).toContain('vpc-one');
    expect(terraformConfig).toContain('sg-one');
    expect(terraformConfig).toContain('sg-two');
    expect(terraformConfig).toContain('vpc-two');
    expect(terraformConfig).toContain('sg-three');

    expect(terraformConfig).toContain('us-west-2');
    expect(terraformConfig).toContain('vpc-three');
    expect(terraformConfig).toContain('sg-four');
  });
});

async function enterDatabaseServiceDetails(
  vpcId = 'vpc-123',
  securityGroupId = 'sg-123'
) {
  await selectEvent.select(
    screen.getByRole('combobox', { name: 'Region' }),
    'US East (N. Virginia)'
  );
  fireEvent.change(screen.getByPlaceholderText('vpc-0123456789abcdef0'), {
    target: { value: vpcId },
  });
  const securityGroupInput = screen.getByRole('combobox', {
    name: 'Security group IDs',
  });
  fireEvent.change(securityGroupInput, {
    target: { value: securityGroupId },
  });
  fireEvent.keyDown(securityGroupInput, { key: 'Enter', code: 'Enter' });
}
