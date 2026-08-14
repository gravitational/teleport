import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

import {
  fireEvent,
  render,
  screen,
  testQueryClient,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport';

import { ScheduledUpgrades } from './ScheduledUpgrades';

beforeEach(() => {
  jest.resetAllMocks();
  testQueryClient.clear();
});

test('displays fetched window and profile values', async () => {
  const ctx = createTeleportContextE();
  ctx.cloudService.getUpgradeWindowStartHour = jest.fn().mockResolvedValue(8);
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'production' });
  ctx.cloudService.updateUpgradeWindowStart = jest.fn().mockResolvedValue(16);
  ctx.cloudService.updateEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'staging' });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(await screen.findByText('08:00 (UTC)')).toBeInTheDocument();
  expect(await screen.findByText('production')).toBeInTheDocument();
});

test('shows loading shimmer while fetching', () => {
  const ctx = createTeleportContextE();
  // never resolves — keeps the component in pending state
  ctx.cloudService.getUpgradeWindowStartHour = jest
    .fn()
    .mockReturnValue(new Promise(() => {}));
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockReturnValue(new Promise(() => {}));

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(screen.queryByText('08:00 (UTC)')).not.toBeInTheDocument();
  expect(screen.queryByTitle('Edit')).not.toBeInTheDocument();
});

test('shows error alert when fetch fails', async () => {
  const ctx = createTeleportContextE();
  ctx.cloudService.getUpgradeWindowStartHour = jest
    .fn()
    .mockRejectedValue(new Error('fetch failed'));
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'production' });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(await screen.findByText(/fetch failed/i)).toBeInTheDocument();
  expect(screen.queryByText('08:00 (UTC)')).not.toBeInTheDocument();
});

test('editing and saving window start time calls update service and displays new value', async () => {
  const ctx = createTeleportContextE();
  ctx.cloudService.getUpgradeWindowStartHour = jest.fn().mockResolvedValue(8);
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'production' });
  // Real API returns EmptyResponse; simulate that here.
  ctx.cloudService.updateUpgradeWindowStart = jest
    .fn()
    .mockResolvedValue(undefined);
  ctx.cloudService.updateEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'staging' });

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  // wait for data to load
  expect(await screen.findByText('08:00 (UTC)')).toBeInTheDocument();

  // open edit mode for Window Start Time
  const editButtons = screen.getAllByTitle('Edit');
  fireEvent.click(editButtons[0]);

  // save button appears
  expect(screen.getByTitle('Save')).toBeInTheDocument();

  // change selection then save
  await selectEvent.select(screen.getByRole('combobox'), '16:00 (UTC)');
  fireEvent.click(screen.getByTitle('Save'));

  await waitFor(() => {
    expect(ctx.cloudService.updateUpgradeWindowStart).toHaveBeenCalledWith(
      expect.any(String),
      16
    );
  });

  // edit mode closes and new value is shown
  await waitFor(() => {
    expect(screen.queryByTitle('Save')).not.toBeInTheDocument();
  });
  expect(screen.getByText('16:00 (UTC)')).toBeInTheDocument();
  expect(screen.queryByText('08:00 (UTC)')).not.toBeInTheDocument();
});

test('shows error inline when save fails', async () => {
  const ctx = createTeleportContextE();
  ctx.cloudService.getUpgradeWindowStartHour = jest.fn().mockResolvedValue(8);
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockResolvedValue({ environmentProfile: 'production' });
  ctx.cloudService.updateUpgradeWindowStart = jest
    .fn()
    .mockRejectedValue(new Error('save failed'));
  ctx.cloudService.updateEnvironmentProfile = jest.fn();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  expect(await screen.findByText('08:00 (UTC)')).toBeInTheDocument();

  const editButtons = screen.getAllByTitle('Edit');
  fireEvent.click(editButtons[0]);
  fireEvent.click(screen.getByTitle('Save'));

  await waitFor(() => {
    expect(screen.getByTestId('window-warn-box')).toBeVisible();
  });
});

// On remount/cache reuse the window query returns 'success' from the cache
// immediately. The prev-value pattern (not a queryFn side-effect) keeps local
// state in sync, so entering edit mode must not throw even before any refetch
// completes.
test('entering edit mode after cache reuse does not throw', () => {
  // Seed the React Query cache so the component sees status=success on the
  // very first render, without queryFn ever running. The clusterId comes from
  // the baseContext fixture used by createTeleportContextE.
  const clusterId = 'aws';
  testQueryClient.setQueryData(['window', clusterId], 8);
  testQueryClient.setQueryData(['env', clusterId], {
    environmentProfile: 'production',
  });

  const ctx = createTeleportContextE();
  // Background refetches will fire (staleTime=0) but must never resolve so the
  // cached values stay visible and no "Query data cannot be undefined" warning
  // is emitted.
  ctx.cloudService.getUpgradeWindowStartHour = jest
    .fn()
    .mockReturnValue(new Promise(() => {}));
  ctx.cloudService.getEnvironmentProfile = jest
    .fn()
    .mockReturnValue(new Promise(() => {}));
  ctx.cloudService.updateUpgradeWindowStart = jest.fn().mockResolvedValue(8);
  ctx.cloudService.updateEnvironmentProfile = jest.fn();

  render(
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <ScheduledUpgrades />
      </ContextProvider>
    </MemoryRouter>
  );

  // Values from cache are visible immediately — no async wait needed.
  expect(screen.getByText('08:00 (UTC)')).toBeInTheDocument();
  expect(screen.getByText('production')).toBeInTheDocument();

  // Clicking Edit must not throw. The prev-value pattern populates the
  // fallback from windowResp.data at render time, no async side effect needed.
  const editButtons = screen.getAllByTitle('Edit');
  expect(() => fireEvent.click(editButtons[0])).not.toThrow();
  expect(screen.getByTitle('Save')).toBeInTheDocument();
});
