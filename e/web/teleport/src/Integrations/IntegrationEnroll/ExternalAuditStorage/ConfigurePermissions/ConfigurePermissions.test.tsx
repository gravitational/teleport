import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import { externalAuditStorageService } from 'e-teleport/services/externalauditstorage';
import TeleportEContext from 'e-teleport/teleportContextE';
import { ContextProvider } from 'teleport';
import { externalAuditStorage } from 'teleport/Integrations/fixtures';
import { allAccessAcl } from 'teleport/mocks/contexts';

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
    setup();
    jest
      .spyOn(externalAuditStorageService, 'getDraft')
      .mockResolvedValue(externalAuditStorage);
    await userEvent.click(screen.getByText('Generate Script'));

    expect(screen.getByText(/Draft in progress/)).toBeInTheDocument();
  });

  it('renders without the warning if no draft exist', async () => {
    setup();
    await userEvent.click(screen.getByText('Generate Script'));

    expect(screen.queryByText(/Draft in progress/)).not.toBeInTheDocument();
  });

  it('renders generate script button when script is null', async () => {
    setup();
    expect(screen.getByText(/Generate Script/)).toBeInTheDocument();
  });
});
