import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';
import { allAccessAcl } from 'teleport/mocks/contexts';

import TeleportEContext from 'e-teleport/teleportContextE';

import { externalAuditStorageService } from 'e-teleport/services/externalauditstorage';

import { ExternalAuditStorageProvider } from '../useExternalAuditStorage';

import { TestConnection } from './TestConnection';

describe('testConnection', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  const setup = () => {
    const ctx = new TeleportEContext();
    ctx.storeUser.setState({
      username: 'joe@example.com',
      acl: allAccessAcl,
    });

    jest
      .spyOn(ctx.externalAuditStorageService, 'testConnection')
      .mockResolvedValue({
        id: 'id',
        success: true,
        message: 'test ok',
        traces: [],
      });

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <ExternalAuditStorageProvider>
            <TestConnection />
          </ExternalAuditStorageProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('calls onTest on button click', async () => {
    await waitFor(() => setup());
    const testFunction = jest.spyOn(
      externalAuditStorageService,
      'testConnection'
    );
    const button = screen.getByTestId(/test-button/);
    await userEvent.click(button);
    expect(testFunction).toHaveBeenCalled();
  });

  it('renders diagnostic traces correctly', async () => {
    await waitFor(() => setup());
    jest
      .spyOn(externalAuditStorageService, 'testConnection')
      .mockResolvedValue({
        id: 'id',
        success: true,
        message: 'ok',
        traces: [
          {
            status: 'failed',
            error: 'Error message',
            details: 'Error details',
            traceType: 'trace-type',
          },
          {
            status: 'success',
            details: 'Success details',
            traceType: 'trace-type',
            error: '',
          },
        ],
      });
    const button = screen.getByTestId(/test-button/);
    await userEvent.click(button);

    expect(screen.getByText(/Error details/)).toBeInTheDocument();
    expect(screen.getByText(/Success details/)).toBeInTheDocument();
  });
});
