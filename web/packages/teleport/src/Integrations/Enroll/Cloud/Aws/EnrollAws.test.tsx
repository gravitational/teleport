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

import { act, fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import 'shared/components/TextEditor/TextEditor.mock';

import { copyToClipboard } from 'design/utils/copyToClipboard';

import { ContextProvider } from 'teleport';
import cfg from 'teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ApiError } from 'teleport/services/api/parseError';
import { integrationService } from 'teleport/services/integrations';

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
  });

  afterEach(() => {
    jest.restoreAllMocks();
    cfg.proxyCluster = defaultProxyCluster;
  });

  test('terraform template renders', async () => {
    renderEnrollAws();

    await waitFor(() => {
      expect(screen.getByText(/module "aws_discovery"/)).toBeInTheDocument();
    });

    const editor = screen.getByTestId('mock-text-editor');
    expect(editor).toHaveTextContent(/module "aws_discovery"/);
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

    fireEvent.change(input, {
      target: { value: 'test-integration' },
    });

    fireEvent.click(copyButton);

    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('teleport_integration_name')
    );

    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('"test-integration"')
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

    const checkButton = screen.getByRole('button', {
      name: /check integration/i,
    });
    fireEvent.click(checkButton);

    const success = await screen.findByText(/successfully added/i);
    expect(success).toBeInTheDocument();

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

    fireEvent.click(screen.getByRole('button', { name: /check integration/i }));

    // wait until polling fails
    await act(async () => {
      await jest.advanceTimersByTimeAsync(35000);
    });

    expect(
      screen.getByRole('button', { name: /^view integration$/i })
    ).toBeDisabled();

    jest.useRealTimers();
  });

  test('panel switches between info and terraform tabs', async () => {
    renderEnrollAws();

    expect(
      screen.getByRole('radio', { name: 'Terraform Configuration' })
    ).toBeChecked();

    const infoButton = screen.getByRole('radio', { name: 'Info Guide' });
    fireEvent.click(infoButton);

    await waitFor(() => {
      expect(screen.getByText(/Reference Links/i)).toBeInTheDocument();
    });

    const terraformButton = screen.getByRole('radio', {
      name: 'Terraform Configuration',
    });
    fireEvent.click(terraformButton);

    await waitFor(() => {
      expect(screen.getByText(/module "aws_discovery"/)).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.queryByText('Reference Links')).not.toBeInTheDocument();
    });
  });
});
