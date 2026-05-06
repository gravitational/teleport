import { http, HttpResponse } from 'msw';
import { PropsWithChildren } from 'react';
import selectEvent from 'react-select-event';

import {
  act,
  enableMswServer,
  Providers,
  render,
  screen,
  server,
  testQueryClient,
  waitFor,
  within,
} from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { TeleportProviderBasicE } from 'e-teleport/mocks/providers';
import cfg from 'teleport/config';
import { mockManagedUpdatesTimeBasedCloud } from 'teleport/ManagedUpdates/fixtures';

import ManagedUpdates from './ManagedUpdates';

enableMswServer();

const originalIsCloud = cfg.isCloud;

beforeEach(() => {
  cfg.isCloud = true;
});

afterEach(() => {
  cfg.isCloud = originalIsCloud;
});

function makeWrapper() {
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContextE();
    return (
      <Providers>
        <TeleportProviderBasicE teleportCtx={ctx}>
          {children}
        </TeleportProviderBasicE>
      </Providers>
    );
  };
}

test('displays window start time and environment profile from API', async () => {
  server.use(
    http.get(cfg.getManagedUpdatesUrl(), () =>
      HttpResponse.json(mockManagedUpdatesTimeBasedCloud)
    ),
    http.get('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'production' })
    )
  );

  render(<ManagedUpdates />, { wrapper: makeWrapper() });

  // Window start time derived from maintenanceStartHour: 3 (unique to ClusterMaintenanceCard)
  await waitFor(() => {
    expect(screen.getByText('03:00 (UTC)')).toBeInTheDocument();
  });

  // Environment profile from the cloud API
  expect(screen.getByText('production')).toBeInTheDocument();
});

test('editing window start time saves and closes edit mode', async () => {
  server.use(
    http.get(cfg.getManagedUpdatesUrl(), () =>
      HttpResponse.json(mockManagedUpdatesTimeBasedCloud)
    ),
    http.get('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'production' })
    ),
    http.post('*/v1/enterprise/sites/*/upgradewindowstart', () =>
      HttpResponse.json({ upgradeWindowStartHour: 3 })
    )
  );

  render(<ManagedUpdates />, { wrapper: makeWrapper() });

  await waitFor(() => {
    expect(screen.getByText('03:00 (UTC)')).toBeInTheDocument();
  });

  const editButtons = screen.getAllByTitle('Edit');
  act(() => editButtons[0].click());

  expect(screen.getByTitle('Save')).toBeInTheDocument();

  screen.getByTitle('Save').click();

  await waitFor(() => {
    expect(screen.queryByTitle('Save')).not.toBeInTheDocument();
  });
});

// window is initialized directly from data.maintenanceStartHour so the value
// is available on the first render without a useEffect cycle. Clicking Edit
// immediately after the card appears must not throw from value.toString().
test('entering window edit mode on first render does not throw', async () => {
  server.use(
    http.get(cfg.getManagedUpdatesUrl(), () =>
      HttpResponse.json(mockManagedUpdatesTimeBasedCloud)
    ),
    http.get('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'production' })
    )
  );

  render(<ManagedUpdates />, { wrapper: makeWrapper() });

  // Wait for the card to appear — this is the first render where WindowInput
  // is mounted and window state must already hold the prop value.
  await waitFor(() => {
    expect(screen.getByText('03:00 (UTC)')).toBeInTheDocument();
  });

  // Click Edit immediately without any further await; before the fix this
  // would throw because the useEffect hadn't run yet.
  const editButtons = screen.getAllByTitle('Edit');
  expect(() => act(() => editButtons[0].click())).not.toThrow();

  // The select renders with the correct initial value from the prop.
  expect(screen.getByTitle('Save')).toBeInTheDocument();
  expect(screen.getByText('03:00 (UTC)')).toBeInTheDocument();
});

test('editing environment profile saves and displays new value', async () => {
  server.use(
    http.get(cfg.getManagedUpdatesUrl(), () =>
      HttpResponse.json(mockManagedUpdatesTimeBasedCloud)
    ),
    http.get('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'production' })
    ),
    http.post('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'staging' })
    )
  );

  render(<ManagedUpdates />, { wrapper: makeWrapper() });

  await waitFor(() => {
    expect(screen.getByText('production')).toBeInTheDocument();
  });

  const editButtons = screen.getAllByTitle('Edit');
  act(() => editButtons[1].click());

  expect(screen.getByTitle('Save')).toBeInTheDocument();

  await selectEvent.select(screen.getByRole('combobox'), 'staging');

  screen.getByTitle('Save').click();

  await waitFor(() => {
    expect(screen.queryByTitle('Save')).not.toBeInTheDocument();
  });

  // new value is shown after save; scope to the profile row to avoid
  // matching the 'staging' rollout group name in the groups table
  const profileRow = screen.getByText('Environment Profile:').parentElement;
  expect(within(profileRow).getByText('staging')).toBeInTheDocument();
  expect(within(profileRow).queryByText('production')).not.toBeInTheDocument();
});

// When the parent refetches and data.maintenanceStartHour changes, the window
// draft is updated during rendering (adjusting-state-during-render pattern) so
// the mutation submits the new value, not the stale one from mount.
test('save uses updated maintenanceStartHour when prop changes after mount', async () => {
  server.use(
    http.get(cfg.getManagedUpdatesUrl(), () =>
      HttpResponse.json(mockManagedUpdatesTimeBasedCloud)
    ),
    http.get('*/v1/enterprise/cloud/environmentprofile', () =>
      HttpResponse.json({ environmentProfile: 'production' })
    )
  );

  render(<ManagedUpdates />, { wrapper: makeWrapper() });

  await waitFor(() => {
    expect(screen.getByText('03:00 (UTC)')).toBeInTheDocument();
  });

  // Simulate parent refetch returning a new maintenanceStartHour (3 → 8).
  testQueryClient.setQueryData(['managed-updates'], {
    ...mockManagedUpdatesTimeBasedCloud,
    clusterMaintenance: {
      ...mockManagedUpdatesTimeBasedCloud.clusterMaintenance,
      maintenanceStartHour: 8,
    },
  });

  await waitFor(() => {
    expect(screen.getByText('08:00 (UTC)')).toBeInTheDocument();
  });

  // Capture the POST body to verify the mutation uses the updated prop value.
  let capturedBody: Record<string, unknown>;
  server.use(
    http.post(
      '*/v1/enterprise/sites/*/upgradewindowstart',
      async ({ request }) => {
        capturedBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ upgradeWindowStartHour: 8 });
      }
    )
  );

  act(() => screen.getAllByTitle('Edit')[0].click());
  screen.getByTitle('Save').click();

  await waitFor(() => {
    expect(capturedBody?.upgradeWindowStartHour).toBe(8);
  });
});
