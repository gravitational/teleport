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

import { userEvent } from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';

import { render, screen, waitFor } from 'design/utils/testing';
import {
  AvailableResourceMode,
  DefaultTab,
  LabelsViewMode,
  ViewMode,
} from 'gen-proto-ts/teleport/userpreferences/v1/unified_resource_preferences_pb';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';
import { makeErrorAttempt } from 'shared/hooks/useAsync';

import { SharedUnifiedResource, UnifiedResourceDefinition } from './types';
import { UnifiedResources, UnifiedResourcesProps } from './UnifiedResources';

const defaultProps: UnifiedResourcesProps = {
  params: { sort: { fieldName: 'name', dir: 'ASC' } },
  resourcesFetchAttempt: { status: 'success' },
  fetchResources: vi.fn().mockResolvedValue(undefined),
  resources: [],
  NoResources: <div>No resources are available.</div>,
  pinning: { kind: 'hidden' },
  availableKinds: [],
  setParams: vi.fn(),
  unifiedResourcePreferences: {
    defaultTab: DefaultTab.ALL,
    viewMode: ViewMode.CARD,
    labelsViewMode: LabelsViewMode.COLLAPSED,
    availableResourceMode: AvailableResourceMode.ACCESSIBLE,
  },
  updateUnifiedResourcesPreferences: vi.fn(),
  onShowStatusInfo: vi.fn(),
};
const node: UnifiedResourceDefinition = {
  kind: 'node',
  subKind: 'teleport',
  tunnel: false,
  id: '104',
  hostname: 'fujedu',
  addr: '172.10.1.20:3022',
  labels: [
    {
      name: 'cluster',
      value: 'one',
    },
    {
      name: 'kernel',
      value: '4.15.0-51-generic',
    },
  ],
};

async function renderUnifiedResources(
  props: Partial<UnifiedResourcesProps> = {}
) {
  const { container } = render(
    <InfoGuidePanelProvider>
      <UnifiedResources
        {...defaultProps}
        {...props}
        params={{ ...defaultProps.params, ...props.params }}
      />
    </InfoGuidePanelProvider>
  );
  await waitFor(() => expect(container).toBeInTheDocument());
}

function resourceWithVisibleContent(): SharedUnifiedResource {
  return {
    resource: node,
    ui: {
      ActionButton: <span>Loaded resource</span>,
    },
  };
}

test('shows error when initial resource loading fails', async () => {
  await renderUnifiedResources({
    resourcesFetchAttempt: {
      status: 'failed',
      statusText: 'connection refused',
    },
  });

  expect(screen.getByText('Could not fetch resources')).toBeInTheDocument();
  expect(screen.getByText('connection refused')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
});

test('shows pinned-resources and view-preferences loading warnings', async () => {
  await renderUnifiedResources({
    pinning: {
      kind: 'supported',
      getClusterPinnedResources: async () => {
        throw new Error('pinned resources unavailable');
      },
      updateClusterPinnedResources: async () => undefined,
    },
    unifiedResourcePreferencesAttempt: makeErrorAttempt(
      new Error('preferences unavailable')
    ),
  });

  expect(
    await screen.findByText(
      'Could not load pinned resources. Pinning is unavailable.'
    )
  ).toBeInTheDocument();
  expect(screen.getByText('pinned resources unavailable')).toBeInTheDocument();
  expect(
    screen.getByText('Could not load view preferences', { exact: false })
  ).toBeInTheDocument();
});

test('hides pinned-resources and view-preferences loading warnings when initial resource loading fails', async () => {
  const ue = userEvent.setup();
  const fetchResources = vi.fn().mockResolvedValue(undefined);
  const retryUnifiedResourcePreferences = vi.fn();
  const getClusterPinnedResources = vi
    .fn()
    .mockRejectedValue(new Error('pinning not available'));

  await renderUnifiedResources({
    fetchResources,
    resourcesFetchAttempt: {
      status: 'failed',
      statusText: 'connection refused',
    },
    retryGetUnifiedResourcePreferences: retryUnifiedResourcePreferences,
    unifiedResourcePreferencesAttempt: makeErrorAttempt(
      new Error('preferences unavailable')
    ),
    pinning: {
      kind: 'supported',
      getClusterPinnedResources,
      updateClusterPinnedResources: async () => undefined,
    },
  });

  expect(screen.getByText('Could not fetch resources')).toBeInTheDocument();
  expect(screen.getByText('connection refused')).toBeInTheDocument();

  expect(
    screen.queryByText('Could not load view preferences', { exact: false })
  ).not.toBeInTheDocument();
  expect(getClusterPinnedResources).toHaveBeenCalledTimes(1);
  expect(
    screen.queryByText('Could not load pinned resources', { exact: false })
  ).not.toBeInTheDocument();

  await ue.click(screen.getByRole('button', { name: 'Retry' }));
  expect(fetchResources).toHaveBeenCalledWith({ force: true });
  expect(retryUnifiedResourcePreferences).toHaveBeenCalledTimes(1);
  expect(getClusterPinnedResources).toHaveBeenCalledTimes(2);
});

test('shows pinned-resources and view-preferences loading warnings when loading more resources fails', async () => {
  const ue = userEvent.setup();
  const fetchResources = vi.fn().mockResolvedValue(undefined);
  const retryUnifiedResourcePreferences = vi.fn();
  const getClusterPinnedResources = vi
    .fn()
    .mockRejectedValue(new Error('pinning not available'));

  await renderUnifiedResources({
    fetchResources,
    resources: [resourceWithVisibleContent()],
    // The first page was loaded, then failed.
    resourcesFetchAttempt: {
      status: 'failed',
      statusText: 'connection refused',
    },
    retryGetUnifiedResourcePreferences: retryUnifiedResourcePreferences,
    unifiedResourcePreferencesAttempt: makeErrorAttempt(
      new Error('preferences unavailable')
    ),
    pinning: {
      kind: 'supported',
      getClusterPinnedResources,
      updateClusterPinnedResources: async () => undefined,
    },
  });

  expect(screen.getByText('Loaded resource')).toBeInTheDocument();
  expect(screen.getByText('Could not fetch resources')).toBeInTheDocument();
  expect(
    screen.getByText('Could not load view preferences', { exact: false })
  ).toBeInTheDocument();
  expect(
    await screen.findByText('Could not load pinned resources', { exact: false })
  ).toBeInTheDocument();
  expect(getClusterPinnedResources).toHaveBeenCalledTimes(1);

  // "Load more" retries only the resources fetch, the warnings have their own retry buttons.
  await ue.click(screen.getByRole('button', { name: 'Load more' }));
  expect(fetchResources).toHaveBeenCalledWith({ force: true });
  expect(retryUnifiedResourcePreferences).not.toHaveBeenCalled();
  expect(getClusterPinnedResources).toHaveBeenCalledTimes(1);
});

test('does not render a retry button for view preferences without a retry callback', async () => {
  await renderUnifiedResources({
    unifiedResourcePreferencesAttempt: makeErrorAttempt(
      new Error('preferences unavailable')
    ),
  });

  expect(
    screen.getByText('Could not load view preferences', { exact: false })
  ).toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Retry' })
  ).not.toBeInTheDocument();
});
