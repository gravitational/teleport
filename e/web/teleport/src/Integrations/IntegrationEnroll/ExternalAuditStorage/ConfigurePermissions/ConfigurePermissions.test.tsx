import React from 'react';
import { MemoryRouter } from 'react-router';
import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { externalAuditStorage } from 'teleport/Integrations/fixtures';

import { ContextProvider } from 'teleport';

import { allAccessAcl } from 'teleport/mocks/contexts';

import TeleportEContext from 'e-teleport/teleportContextE';

import { externalAuditStorageService } from 'e-teleport/services/externalauditstorage';

import { ExternalAuditStorageProvider } from '../useExternalAuditStorage';

import { ConfigurePermissions } from './ConfigurePermissions';

describe('configurePermissions', () => {
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
      .spyOn(ctx.externalAuditStorageService, 'getDraft')
      .mockResolvedValue(null);

    jest
      .spyOn(ctx.externalAuditStorageService, 'generateDraft')
      .mockResolvedValue(externalAuditStorage);

    render(
      <MemoryRouter>
        <ContextProvider ctx={ctx}>
          <ExternalAuditStorageProvider>
            <ConfigurePermissions />
          </ExternalAuditStorageProvider>
        </ContextProvider>
      </MemoryRouter>
    );
  };

  it('renders warning if a previous draft exists', async () => {
    await waitFor(() => setup());
    jest
      .spyOn(externalAuditStorageService, 'getDraft')
      .mockResolvedValue(externalAuditStorage);
    await userEvent.click(screen.getByText('Generate Script'));

    expect(screen.getByText(/Draft in progress/)).toBeInTheDocument();
  });

  it('renders without the warning if no draft exist', async () => {
    await waitFor(() => setup());
    await userEvent.click(screen.getByText('Generate Script'));

    expect(screen.queryByText(/Draft in progress/)).not.toBeInTheDocument();
    expect(screen.getByText(/Configure Permissions/)).toBeInTheDocument();
  });

  it('renders generate script button when script is null', async () => {
    await waitFor(() => setup());
    expect(screen.getByText(/Generate Script/)).toBeInTheDocument();
  });
});
