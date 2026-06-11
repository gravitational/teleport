import { http, HttpResponse } from 'msw';
import { generatePath, MemoryRouter } from 'react-router';

import {
  createDeferredResponse,
  enableMswServer,
  render,
  screen,
  server,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import cfg from 'e-teleport/config';
import {
  RecordingSummaryState,
  type SessionRecordingSummaryResponse,
} from 'e-teleport/services/recordings/types';
import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { ViewSummary } from './ViewSummary';

enableMswServer();

beforeEach(() => {
  server.use(
    http.get(cfg.oss.api.clustersPath, () => {
      return HttpResponse.json([
        {
          name: 'teleport',
          lastConnected: '2025-08-14T14:36:07.976470934Z',
          status: 'online',
          publicURL: '',
          authVersion: '',
          proxyVersion: '',
        },
      ]);
    })
  );
});
afterEach(() => {
  testQueryClient.clear();
});

const mockSessionId = 'test-session-123';

const getSummaryUrl = generatePath(cfg.api.sessionRecordingSummary, {
  clusterId: 'localhost',
  sessionId: mockSessionId,
});

function withRecordingSummary(summary: SessionRecordingSummaryResponse) {
  server.use(
    http.get(getSummaryUrl, () => {
      return HttpResponse.json(summary);
    })
  );
}

function setupTest() {
  const ctx = createTeleportContext();

  return render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ViewSummary
          durationMs={10000}
          recordingType="ssh"
          username="test"
          hostname="test-server"
          createdDate={new Date()}
          sessionId={mockSessionId}
        />
      </ContextProvider>
    </MemoryRouter>
  );
}

describe('rendering', () => {
  it('renders the summary button', () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });

    expect(button).toBeInTheDocument();
  });

  it('shows tooltip on hover', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.hover(button);

    const tooltip = await screen.findByTestId('tooltip');

    expect(tooltip).toBeInTheDocument();
    expect(tooltip).toHaveTextContent('View session summary');
  });
});

describe('popover behavior', () => {
  it('opens popover when button is clicked', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('Test content')).toBeInTheDocument();
    expect(screen.getByText(/AI can make mistakes/)).toBeInTheDocument();
  });

  it('closes popover when clicking outside', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('Test content')).toBeInTheDocument();

    await userEvent.click(document.body);

    await waitFor(() => {
      expect(screen.queryByText('Test content')).not.toBeInTheDocument();
    });
  });

  it('closes popover when pressing Escape', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('Test content')).toBeInTheDocument();

    await userEvent.keyboard('{Escape}');

    await waitFor(() => {
      expect(screen.queryByText('Test content')).not.toBeInTheDocument();
    });
  });
});

describe('summary states', () => {
  it('displays loading state', async () => {
    const deferred = createDeferredResponse({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: 'Test content',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    server.use(http.get(getSummaryUrl, deferred.handler));

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    // While loading, the popover is not shown yet (only shows when data or error is available)
    expect(screen.queryByText('Test content')).not.toBeInTheDocument();

    deferred.resolve();

    expect(await screen.findByText('Test content')).toBeInTheDocument();
  });

  it('displays pending state when summary is being generated', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Pending,
      inferenceStartedAt: new Date(Date.now() - 1000).toISOString(),
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(
      await screen.findByText('Session summary is currently being generated')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument();
  });

  it('displays a nice message when summary generation fails due to the session being too long', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Error,
      errorMessage: 'session transcript exceeds maximum length',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(
      await screen.findByText(
        'This session was not summarized because it is too large.'
      )
    ).toBeInTheDocument();
  });

  it('displays error state when summary generation fails', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Error,
      errorMessage: 'Some error message about it failing to generate',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(
      await screen.findByText(
        /There was an error generating the session summary/
      )
    ).toBeInTheDocument();

    expect(
      screen.getByText('Some error message about it failing to generate')
    ).toBeInTheDocument();
  });

  it('displays success state with markdown content', async () => {
    const markdownContent = `# Session Overview
## Key Actions
- User logged in to server
- Executed diagnostic commands
- Updated configuration files

## Summary
The session involved routine maintenance tasks.`;

    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: markdownContent,
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('Session Overview')).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: 'Key Actions' })
    ).toBeInTheDocument();
    expect(screen.getByText(/User logged in to server/)).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { name: 'Summary' })
    ).toBeInTheDocument();
    expect(
      screen.getByText('The session involved routine maintenance tasks.')
    ).toBeInTheDocument();
  });

  it('handles error state with retry functionality', async () => {
    jest.spyOn(console, 'error').mockImplementation();

    server.use(
      http.get(getSummaryUrl, () => {
        return HttpResponse.json(
          { error: { message: 'Network error' } },
          { status: 500 }
        );
      })
    );

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(
      await screen.findByText('Error loading session summary')
    ).toBeInTheDocument();
    expect(screen.getByText('Network error')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();

    server.use(
      http.get(getSummaryUrl, () => {
        return HttpResponse.json({
          sessionId: mockSessionId,
          state: RecordingSummaryState.Success,
          content: '# Recovered Summary',
          inferenceStartedAt: '2025-01-15T10:00:00Z',
          inferenceFinishedAt: '2025-01-15T10:01:00Z',
        });
      })
    );

    const retryButton = screen.getByRole('button', { name: 'Retry' });
    await userEvent.click(retryButton);

    await waitFor(() => {
      expect(
        screen.queryByText('Error loading session summary')
      ).not.toBeInTheDocument();
    });

    expect(await screen.findByText('Recovered Summary')).toBeInTheDocument();
  });
});

describe('refetch functionality', () => {
  it('allows refetching when summary is pending', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Pending,
      inferenceStartedAt: new Date().toISOString(),
    });

    setupTest();

    const button = screen.getByRole('button');
    await userEvent.click(button);

    expect(
      await screen.findByRole('button', { name: 'Reload' })
    ).toBeInTheDocument();

    server.use(
      http.get(getSummaryUrl, () => {
        return HttpResponse.json({
          sessionId: mockSessionId,
          state: RecordingSummaryState.Success,
          content: '# Completed Summary',
          inferenceStartedAt: '2025-01-15T10:00:00Z',
          inferenceFinishedAt: '2025-01-15T10:01:00Z',
        });
      })
    );

    const refetchButton = screen.getByRole('button', {
      name: 'Reload',
    });
    await userEvent.click(refetchButton);

    await waitFor(() => {
      expect(
        screen.queryByText(/Session summary is currently being generated/)
      ).not.toBeInTheDocument();
    });

    expect(await screen.findByText('Completed Summary')).toBeInTheDocument();
  });

  it('disables refetch button while refetching', async () => {
    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Pending,
      inferenceStartedAt: new Date().toISOString(),
    });

    setupTest();

    const button = screen.getByRole('button');
    await userEvent.click(button);

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: 'Reload' })
      ).toBeInTheDocument();
    });

    const deferred = createDeferredResponse({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      content: '# Completed Summary',
      inferenceStartedAt: '2025-01-15T10:00:00Z',
      inferenceFinishedAt: '2025-01-15T10:01:00Z',
    });

    server.use(http.get(getSummaryUrl, deferred.handler));

    const refetchButton = screen.getByRole('button', {
      name: 'Reload',
    });
    await userEvent.click(refetchButton);

    expect(
      await screen.findByRole('button', { name: /Reloading.../i })
    ).toBeDisabled();

    deferred.resolve();

    expect(await screen.findByText('Completed Summary')).toBeInTheDocument();
  });
});

describe('duration', () => {
  it('shows the duration of the summarization when complete', async () => {
    const now = new Date();
    const fiveMinutesAgo = new Date(now.getTime() - 5 * 60 * 1000);

    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      inferenceStartedAt: fiveMinutesAgo.toISOString(),
      inferenceFinishedAt: now.toISOString(),
      content: 'This is a summary.',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('This is a summary.')).toBeInTheDocument();

    expect(
      screen.getByText('Summarization took 5 minutes.')
    ).toBeInTheDocument();
  });

  it('handles displaying long durations correctly', async () => {
    const now = new Date();
    const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000);

    withRecordingSummary({
      sessionId: mockSessionId,
      state: RecordingSummaryState.Success,
      inferenceStartedAt: twoHoursAgo.toISOString(),
      inferenceFinishedAt: now.toISOString(),
      content: 'This is a summary.',
    });

    setupTest();

    const button = screen.getByRole('button', {
      name: 'View session summary',
    });
    await userEvent.click(button);

    expect(await screen.findByText('This is a summary.')).toBeInTheDocument();

    expect(screen.getByText('Summarization took 2 hours.')).toBeInTheDocument();
  });
});
