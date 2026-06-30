import { QueryClientProvider } from '@tanstack/react-query';
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
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import {
  listBeamsError,
  listBeamsSuccess,
} from 'e-teleport/test/helpers/beams';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import { beamsService } from '../services/beams/beams';
import { BeamsList } from './BeamsList';

jest.mock('../services/beams/beams', () => {
  const actual = jest.requireActual('../services/beams/beams');
  return {
    beamsService: {
      ...actual.beamsService,
      listBeams: jest.fn((...all) => {
        return actual.beamsService.listBeams(...all);
      }),
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
      screen.getByText('Create your first beam using', { exact: false })
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
    expect(
      screen.getByText('a0f98569-2559-42e0-8ac3-2bc6bf5db6c9')
    ).toBeInTheDocument();
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
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
        sortDir: 'ASC',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    fireEvent.click(screen.getByText('UUID'));

    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenCalledTimes(2);
    });
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'name',
        sortDir: 'DESC',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );

    fireEvent.click(screen.getByText('UUID'));

    await waitFor(() => {
      expect(beamsService.listBeams).toHaveBeenCalledTimes(3);
    });
    expect(beamsService.listBeams).toHaveBeenLastCalledWith(
      {
        pageSize: 20,
        pageToken: '',
        sortField: 'name',
        sortDir: 'ASC',
        users: ['llama'],
      },
      expect.any(String),
      expect.any(AbortSignal)
    );
  });
});

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
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({
      customAcl,
    });

    return (
      <QueryClientProvider client={testQueryClient}>
        <ConfiguredThemeProvider theme={theme}>
          <TeleportProviderBasic
            teleportCtx={ctx}
            initialEntries={[cfg.routes.beamsList]}
          >
            {children}
          </TeleportProviderBasic>
        </ConfiguredThemeProvider>
      </QueryClientProvider>
    );
  };
}
