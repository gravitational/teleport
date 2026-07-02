import { PropsWithChildren } from 'react';

import { ConfiguredThemeProvider } from 'design/ThemeProvider';
import {
  enableMswServer,
  fireEvent,
  render,
  screen,
  server,
  testQueryClient,
  theme,
  waitFor,
  waitForElementToBeRemoved,
  within,
} from 'design/utils/testing';

import { Beam } from 'e-teleport/services/beams/types';
import {
  BeamsProviders,
  deleteBeamSuccess,
  listBeamsError,
  listBeamsSuccess,
  updateBeamSuccess,
} from 'e-teleport/test/helpers/beams';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import { beamsService } from '../services/beams/beams';
import { BeamsList } from './BeamsList';

jest.mock('../services/beams/beams', () => {
  const actual = jest.requireActual('../services/beams/beams');
  return {
    beamsService: {
      ...actual.beamsService,
      listBeams: jest.fn((...all) => actual.beamsService.listBeams(...all)),
      createBeam: jest.fn((...all) => actual.beamsService.createBeam(...all)),
      getBeam: jest.fn((...all) => actual.beamsService.getBeam(...all)),
      updateBeam: jest.fn((...all) => actual.beamsService.updateBeam(...all)),
      deleteBeam: jest.fn((...all) => actual.beamsService.deleteBeam(...all)),
    },
  };
});

enableMswServer();

afterEach(async () => {
  await testQueryClient.resetQueries();
  jest.clearAllMocks();
});

describe('BeamsList', () => {
  it('shows an empty state', async () => {
    server.use(
      listBeamsSuccess({
        items: [],
        next_page_token: '',
      })
    );

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('No beams found')).toBeInTheDocument();
    expect(
      screen.getByText('Create your first beam with', { exact: false })
    ).toBeInTheDocument();
  });

  it('shows an error state', async () => {
    server.use(listBeamsError(500, 'server error'));

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('server error')).toBeInTheDocument();
  });

  it('shows an unsupported sort error state', async () => {
    const testErrorMessage =
      'unsupported sort, only name:asc is supported, but got "name" (desc = true)';
    server.use(listBeamsError(400, testErrorMessage));

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    server.use(listBeamsSuccess());

    const resetButton = screen.getByText('Reset sort');
    expect(resetButton).toBeInTheDocument();
    fireEvent.click(resetButton);

    await waitFor(() => {
      expect(screen.queryByText(testErrorMessage)).not.toBeInTheDocument();
    });
  });

  it('shows an unauthorized error state', () => {
    server.use(listBeamsSuccess());

    render(<BeamsList />, {
      wrapper: makeWrapper(
        makeAcl({
          beam: {
            ...defaultAccess,
            list: false,
            read: false,
          },
        })
      ),
    });

    expect(
      screen.getByText(
        'You do not have permission to access Beams. Missing role permissions:',
        { exact: false }
      )
    ).toBeInTheDocument();
    expect(screen.getByText('beams.list')).toBeInTheDocument();
    expect(screen.getByText('beams.read')).toBeInTheDocument();
  });

  it('shows a list', async () => {
    server.use(listBeamsSuccess());

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByText('cosmic-author')).toBeInTheDocument();
    expect(screen.getByText('solid-flux')).toBeInTheDocument();
  });

  it('allows paging', async () => {
    jest
      .mocked(beamsService.listBeams)
      .mockImplementation(async ({ pageToken }) => ({
        items: [
          {
            name: 'a0f98569-2559-42e0-8ac3-2bc6bf5db6c9',
            alias: 'cosmic-author',
            expires: '2026-04-25T16:30:00Z',
            user: 'user@example.com',
            node_id: '',
            app_name: '',
            egress_mode: 'unrestricted',
            allowed_domains: [],
            compute_status: 'provision_complete',
          },
        ],
        next_page_token: pageToken + '.next',
      }));

    expect(beamsService.listBeams).toHaveBeenCalledTimes(0);

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    const [nextButton] = screen.getAllByTitle('Next page');

    expect(beamsService.listBeams).toHaveBeenCalledTimes(1);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(beamsService.listBeams).toHaveBeenCalledTimes(2);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '.next',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(beamsService.listBeams).toHaveBeenCalledTimes(3);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '.next.next',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    const [prevButton] = screen.getAllByTitle('Previous page');

    await waitFor(() => expect(prevButton).toBeEnabled());
    fireEvent.click(prevButton);

    // Previous pages are cached
    expect(beamsService.listBeams).toHaveBeenCalledTimes(3);

    await waitFor(() => expect(prevButton).toBeEnabled());
    fireEvent.click(prevButton);

    // Previous pages are cached
    expect(beamsService.listBeams).toHaveBeenCalledTimes(3);
  });

  it('allows filtering by own beams', async () => {
    jest
      .mocked(beamsService.listBeams)
      .mockImplementation(async ({ pageToken, users }) => ({
        items: [
          {
            name: `beam-${pageToken || 'root'}-${users?.join(',') || 'all'}`,
            alias: 'cosmic-author',
            expires: '2026-04-25T16:30:00Z',
            user: 'user@example.com',
            node_id: '',
            app_name: '',
            egress_mode: 'unrestricted',
            allowed_domains: [],
            compute_status: 'provision_complete',
          },
        ],
        next_page_token: pageToken + '.next',
      }));

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(beamsService.listBeams).toHaveBeenCalledTimes(1);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );
    expect(screen.queryByText('user@example.com')).not.toBeInTheDocument();

    const [nextButton] = screen.getAllByTitle('Next page');
    await waitFor(() => expect(nextButton).toBeEnabled());
    fireEvent.click(nextButton);

    expect(beamsService.listBeams).toHaveBeenCalledTimes(2);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '.next',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    fireEvent.click(screen.getByTestId('toggle'));

    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenCalledTimes(3);
    });
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'expires',
        sortDir: 'asc',
        users: undefined,
      },
      expect.any(String),
      expect.any(AbortSignal)
    );
    expect(screen.getByText('user@example.com')).toBeInTheDocument();
  });

  it('allows sorting', async () => {
    jest
      .mocked(beamsService.listBeams)
      .mockImplementation(async ({ pageToken }) => ({
        items: [
          {
            name: 'a0f98569-2559-42e0-8ac3-2bc6bf5db6c9',
            alias: 'cosmic-author',
            expires: '2026-04-25T16:30:00Z',
            user: 'user@example.com',
            node_id: '',
            app_name: '',
            egress_mode: 'unrestricted',
            allowed_domains: [],
            compute_status: 'provision_complete',
          },
        ],
        next_page_token: pageToken,
      }));

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(beamsService.listBeams).toHaveBeenCalledTimes(1);
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'expires',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    fireEvent.click(screen.getByText('ID'));

    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenCalledTimes(2);
    });
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'alias',
        sortDir: 'desc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    fireEvent.click(screen.getByText('ID'));

    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenCalledTimes(3);
    });
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'alias',
        sortDir: 'asc',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );
  });

  it('creates a beam directly, re-sorts by expires DESC, and shows the new row', async () => {
    let beamsList: Beam[] = [];
    jest.mocked(beamsService.listBeams).mockImplementation(async () => ({
      items: beamsList,
      next_page_token: '',
    }));
    jest.mocked(beamsService.createBeam).mockImplementation(async () => {
      beamsList = [ownedBeam];
      return ownedBeam;
    });

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.queryByText(ownedBeam.alias)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /create beam/i }));

    await waitFor(() => {
      expect(beamsService.createBeam).toHaveBeenCalledWith(expect.any(String));
    });
    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenLastCalledWith(
        expect.objectContaining({ sortField: 'expires', sortDir: 'desc' }),
        expect.any(String),
        expect.any(AbortSignal)
      );
    });
    await waitFor(
      () => {
        expect(screen.getByText(ownedBeam.alias)).toBeInTheDocument();
      },
      { timeout: 2000 }
    );
  });

  it('publishes an owned beam from the row menu', async () => {
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [ownedBeam],
      next_page_token: '',
    });
    const published = {
      ...ownedBeam,
      app_name: 'my-beam-1111',
      publish: { port: 8080, protocol: 'http' as const },
    };
    server.use(updateBeamSuccess(published));

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    fireEvent.click(screen.getByTestId('button')); // overflow menu
    fireEvent.click(screen.getByRole('menuitem', { name: /publish/i }));

    await waitFor(() => {
      expect(beamsService.updateBeam).toHaveBeenCalledWith(
        { clusterId: expect.any(String), name: ownedBeam.name },
        expect.objectContaining({
          publish: { port: 8080, protocol: 'http' },
        })
      );
    });
  });

  it('unpublishes an owned beam from the row menu', async () => {
    const publishedBeam = {
      ...ownedBeam,
      app_name: 'my-beam-1111',
      publish: { port: 8080, protocol: 'http' as const },
    };
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [publishedBeam],
      next_page_token: '',
    });
    const unpublished = { ...publishedBeam, publish: undefined };
    server.use(updateBeamSuccess(unpublished));

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    fireEvent.click(screen.getByTestId('button')); // overflow menu
    fireEvent.click(screen.getByRole('menuitem', { name: /unpublish/i }));

    await waitFor(() => {
      expect(beamsService.updateBeam).toHaveBeenCalledWith(
        { clusterId: expect.any(String), name: ownedBeam.name },
        expect.objectContaining({ publish: undefined })
      );
    });
  });

  it('disables connect and publish for non-owners while leaving delete interactive', async () => {
    // doesn't belong to the logged-in user.
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [{ ...ownedBeam, user: 'other-user' }],
      next_page_token: '',
    });

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByRole('button', { name: /connect/i })).toBeDisabled();

    fireEvent.click(screen.getByTestId('button'));

    fireEvent.click(screen.getByRole('menuitem', { name: /publish/i }));
    expect(beamsService.updateBeam).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('menuitem', { name: /delete/i }));
    expect(await screen.findByText(/delete beam\?/i)).toBeInTheDocument();
  });

  it('disables row actions while a beam is provisioning', async () => {
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [provisioningBeam],
      next_page_token: '',
    });

    render(<BeamsList />, { wrapper: makeWrapper() });

    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(screen.getByRole('button', { name: /connect/i })).toBeDisabled();
    expect(screen.queryByTestId('button')).not.toBeInTheDocument();
  });

  it('selects a beam, opens the confirm dialog from the controls Delete button, and deletes on confirm', async () => {
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [ownedBeam],
      next_page_token: '',
    });
    server.use(deleteBeamSuccess());

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    expect(
      screen.queryByRole('button', { name: /^delete \(/i })
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole('checkbox', {
        name: new RegExp(`select beam ${ownedBeam.alias}`, 'i'),
      })
    );

    const deleteButton = await screen.findByRole('button', {
      name: /^delete \(1\)$/i,
    });
    fireEvent.click(deleteButton);

    const dialog = await screen.findByRole('dialog');
    fireEvent.click(
      within(dialog).getByRole('button', {
        name: /^i understand, delete beam$/i,
      })
    );

    await waitFor(() => {
      expect(beamsService.deleteBeam).toHaveBeenCalledWith({
        clusterId: expect.any(String),
        name: ownedBeam.name,
      });
    });
  });

  it('bulk-deletes multiple beams selected via individual row checkboxes', async () => {
    const second: Beam = {
      ...ownedBeam,
      name: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
      alias: 'second-beam',
    };
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [ownedBeam, second],
      next_page_token: '',
    });
    server.use(deleteBeamSuccess());

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    fireEvent.click(
      screen.getByRole('checkbox', {
        name: new RegExp(`select beam ${ownedBeam.alias}`, 'i'),
      })
    );
    fireEvent.click(
      screen.getByRole('checkbox', {
        name: new RegExp(`select beam ${second.alias}`, 'i'),
      })
    );

    fireEvent.click(
      await screen.findByRole('button', { name: /^delete \(2\)$/i })
    );

    const dialog = await screen.findByRole('dialog');
    fireEvent.click(
      within(dialog).getByRole('button', {
        name: /^i understand, delete beams$/i,
      })
    );

    await waitFor(() => {
      expect(beamsService.deleteBeam).toHaveBeenCalledTimes(2);
    });
    expect(beamsService.deleteBeam).toHaveBeenCalledWith({
      clusterId: expect.any(String),
      name: ownedBeam.name,
    });
    expect(beamsService.deleteBeam).toHaveBeenCalledWith({
      clusterId: expect.any(String),
      name: second.name,
    });
  });

  it('preserves selection across page navigation and lets the user bulk-delete', async () => {
    const beamOnPage1 = ownedBeam;
    const beamOnPage2: Beam = {
      ...ownedBeam,
      name: 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
      alias: 'page-two-beam',
    };
    jest
      .mocked(beamsService.listBeams)
      .mockImplementation(async ({ pageToken }) => ({
        items: pageToken ? [beamOnPage2] : [beamOnPage1],
        next_page_token: pageToken ? '' : 'p2',
      }));
    server.use(deleteBeamSuccess());

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    // Select the beam on page 1.
    fireEvent.click(
      screen.getByRole('checkbox', {
        name: new RegExp(`select beam ${beamOnPage1.alias}`, 'i'),
      })
    );
    expect(
      screen.getByRole('button', { name: /^delete \(1\)$/i })
    ).toBeInTheDocument();

    // Navigate to page 2.
    const [nextButton] = screen.getAllByTitle('Next page');
    fireEvent.click(nextButton);
    await screen.findByText(beamOnPage2.alias);

    // Selection from page 1 still counts.
    expect(
      screen.getByRole('button', { name: /^delete \(1\)$/i })
    ).toBeInTheDocument();

    // Also select the beam on page 2 — counter ticks to 2.
    fireEvent.click(
      screen.getByRole('checkbox', {
        name: new RegExp(`select beam ${beamOnPage2.alias}`, 'i'),
      })
    );
    fireEvent.click(
      await screen.findByRole('button', { name: /^delete \(2\)$/i })
    );

    const dialog = await screen.findByRole('dialog');
    fireEvent.click(
      within(dialog).getByRole('button', {
        name: /^i understand, delete beams$/i,
      })
    );

    await waitFor(() => {
      expect(beamsService.deleteBeam).toHaveBeenCalledTimes(2);
    });
    expect(beamsService.deleteBeam).toHaveBeenCalledWith({
      clusterId: expect.any(String),
      name: beamOnPage1.name,
    });
    expect(beamsService.deleteBeam).toHaveBeenCalledWith({
      clusterId: expect.any(String),
      name: beamOnPage2.name,
    });
  });

  it('deletes a beam via the row-menu Delete... item with the confirm dialog', async () => {
    jest.mocked(beamsService.listBeams).mockResolvedValue({
      items: [ownedBeam],
      next_page_token: '',
    });
    server.use(deleteBeamSuccess());

    render(<BeamsList />, { wrapper: makeWrapper() });
    await waitForElementToBeRemoved(() => screen.queryByTestId('loading'));

    fireEvent.click(screen.getByTestId('button'));
    fireEvent.click(screen.getByRole('menuitem', { name: /delete/i }));

    const dialog = await screen.findByRole('dialog');
    fireEvent.click(
      within(dialog).getByRole('button', {
        name: /^i understand, delete beam$/i,
      })
    );

    await waitFor(() => {
      expect(beamsService.deleteBeam).toHaveBeenCalledWith({
        clusterId: expect.any(String),
        name: ownedBeam.name,
      });
    });
  });
});

const ownedBeam: Beam = {
  name: '11111111-2222-3333-4444-555555555555',
  alias: 'my-beam',
  expires: '2027-01-01T00:00:00Z',
  user: 'llama',
  node_id: 'node-1',
  app_name: '',
  egress_mode: 'unrestricted',
  allowed_domains: [],
  compute_status: 'provision_complete',
};

const provisioningBeam: Beam = {
  ...ownedBeam,
  name: '99999999-aaaa-bbbb-cccc-dddddddddddd',
  alias: 'pending-beam',
  node_id: '',
  compute_status: 'provision_pending',
};

function makeWrapper(
  customAcl: ReturnType<typeof makeAcl> = makeAcl({
    beam: {
      list: true,
      create: true,
      edit: true,
      remove: true,
      read: true,
    },
  })
) {
  return ({ children }: PropsWithChildren) => (
    <ConfiguredThemeProvider theme={theme}>
      <BeamsProviders acl={customAcl} queryClient={testQueryClient}>
        {children}
      </BeamsProviders>
    </ConfiguredThemeProvider>
  );
}
