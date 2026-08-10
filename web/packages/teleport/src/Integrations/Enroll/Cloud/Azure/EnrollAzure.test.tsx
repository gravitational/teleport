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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from 'teleport';
import { ContentMinWidth } from 'teleport/Main/Main';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { userEventService } from 'teleport/services/userEvent';

import { EnrollAzure } from './EnrollAzure';

describe('EnrollAzure', () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  function renderEnrollAzure() {
    const ctx = createTeleportContext();
    ctx.storeUser.state.cluster.authVersion = '1.0.0';

    return render(
      <ContextProvider ctx={ctx}>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter>
            <InfoGuidePanelProvider>
              <ContentMinWidth>
                <EnrollAzure />
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
    jest
      .spyOn(userEventService, 'captureIntegrationEnrollEvent')
      .mockImplementation();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  test('does not contain any AWS references', () => {
    const { container } = renderEnrollAzure();
    expect(container).not.toHaveTextContent(/\bAWS\b/);
  });

  test('validates integration name is required', async () => {
    renderEnrollAzure();

    const input = screen.getByLabelText(/integration name/i);
    fireEvent.change(input, { target: { value: '' } });

    const copyButtons = screen.getAllByRole('button', {
      name: /copy terraform module/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => {
      expect(
        screen.getByText(/integration name is required/i)
      ).toBeInTheDocument();
    });
  });

  test('validates resource group name is required', async () => {
    renderEnrollAzure();

    const copyButtons = screen.getAllByRole('button', {
      name: /copy terraform module/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => {
      expect(
        screen.getByText(/resource group name is required/i)
      ).toBeInTheDocument();
    });
  });

  describe('scope', () => {
    describe('management group', () => {
      test('is selected by default', () => {
        renderEnrollAzure();
        expect(screen.getByLabelText(/^management group$/i)).toBeChecked();
        expect(
          screen.getByLabelText(/management group id/i)
        ).toBeInTheDocument();
      });

      test('subscription matcher is not required', () => {
        renderEnrollAzure();
        expect(screen.getByText(/match subscriptions/i)).not.toHaveTextContent(
          '*'
        );
      });
    });

    describe('subscription', () => {
      test('shows subscription ID field instead of management group ID', () => {
        renderEnrollAzure();
        fireEvent.click(screen.getByLabelText(/^subscription$/i));
        expect(screen.getByLabelText(/subscription id/i)).toBeInTheDocument();
        expect(
          screen.queryByLabelText(/management group id/i)
        ).not.toBeInTheDocument();
      });

      test('subscription ID mirrors first subscription matcher', () => {
        renderEnrollAzure();
        fireEvent.click(screen.getByLabelText(/^subscription$/i));
        const input = screen.getByLabelText(/subscription id/i);
        fireEvent.change(input, {
          target: { value: '11111111-2222-3333-4444-555555555555' },
        });

        const matchers = screen.getAllByPlaceholderText(
          '11111111-2222-3333-4444-555555555555'
        );
        expect(matchers[0]).toHaveValue('11111111-2222-3333-4444-555555555555');
      });

      test('validates subscription ID is required', async () => {
        renderEnrollAzure();
        fireEvent.click(screen.getByLabelText(/^subscription$/i));
        const copyButtons = screen.getAllByRole('button', {
          name: /copy terraform module/i,
        });
        fireEvent.click(copyButtons[0]);

        await waitFor(() => {
          expect(
            screen.getByText(/subscription id is required/i)
          ).toBeInTheDocument();
        });
      });
    });
  });
});
