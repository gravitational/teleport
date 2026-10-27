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

  expect(screen.getByText('Resources could not be loaded')).toBeInTheDocument();
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

  expect(screen.getByText('Resources could not be loaded')).toBeInTheDocument();
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

test('shows the default empty state when there are no resources', async () => {
  await renderUnifiedResources();

  expect(screen.getByText('No resources')).toBeInTheDocument();
  expect(
    screen.getByText(
      "Either there are no resources in the cluster, or your roles don't grant you access."
    )
  ).toBeInTheDocument();
});

test('does not show an empty state while resources are loading', async () => {
  await renderUnifiedResources({
    resourcesFetchAttempt: { status: 'processing' },
  });

  expect(screen.queryByText('No resources')).not.toBeInTheDocument();
});

test('does not show an empty state when there are resources', async () => {
  await renderUnifiedResources({ resources: [resourceWithVisibleContent()] });

  expect(screen.getByText('Loaded resource')).toBeInTheDocument();
  expect(screen.queryByText('No resources')).not.toBeInTheDocument();
});

test.each([
  { params: {}, state: 'all resources' },
  { params: { pinnedOnly: true }, state: 'pinned resources' },
  { params: { search: 'foo' }, state: 'search' },
  { params: { query: 'name == "foo"' }, state: 'predicate query' },
])(
  'shows the failure empty state instead of $state empty content',
  async ({ params }) => {
    const noMatches = vi.fn().mockReturnValue(<div>Custom matches panel</div>);
    await renderUnifiedResources({
      params,
      resourcesFetchAttempt: { status: 'failed', statusText: 'boom' },
      emptyState: {
        noResources: <div>Custom empty panel</div>,
        noPinned: <div>Custom pinned panel</div>,
        noMatches,
      },
    });

    expect(
      screen.getByRole('heading', { name: 'Resources could not be loaded' })
    ).toBeVisible();
    expect(screen.getByText('boom')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Retry' })).toBeVisible();
    expect(
      screen.queryByText('Could not fetch resources')
    ).not.toBeInTheDocument();
    expect(screen.queryByText('No resources')).not.toBeInTheDocument();
    expect(screen.queryByText('Pinned resources')).not.toBeInTheDocument();
    expect(screen.queryByText(/No resources match/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Custom .* panel/)).not.toBeInTheDocument();
    expect(noMatches).not.toHaveBeenCalled();
  }
);

test.each([400, 403])(
  'does not offer retry when resource loading fails with HTTP %i',
  async statusCode => {
    await renderUnifiedResources({
      resourcesFetchAttempt: {
        status: 'failed',
        statusCode,
        statusText: 'request rejected',
      },
    });

    expect(
      screen.getByRole('heading', { name: 'Resources could not be loaded' })
    ).toBeVisible();
    expect(screen.getByText('request rejected')).toBeVisible();
    expect(
      screen.queryByRole('button', { name: 'Retry' })
    ).not.toBeInTheDocument();
  }
);

test('shows a no-matches empty state for a search', async () => {
  await renderUnifiedResources({ params: { search: 'foo' } });

  expect(await screen.findByText('No resources match "foo"')).toBeVisible();
  expect(
    screen.getByText('Check the query or try a new search.')
  ).toBeInTheDocument();
});

test('shows a no-matches empty state for a predicate query', async () => {
  await renderUnifiedResources({ params: { query: 'name == "foo"' } });

  expect(
    await screen.findByText('No resources match "name == "foo""')
  ).toBeVisible();
});

test('mentions that only pinned resources are searched on the pinned tab', async () => {
  await renderUnifiedResources({
    params: { search: 'foo', pinnedOnly: true },
  });

  expect(await screen.findByText('No resources match "foo"')).toBeVisible();
  expect(
    screen.getByText(
      'Only pinned resources are searched. Check the query or try a new search.'
    )
  ).toBeInTheDocument();
});

test('shows a dedicated empty state on the pinned tab without a search', async () => {
  await renderUnifiedResources({ params: { pinnedOnly: true } });

  expect(
    screen.getByRole('heading', { name: 'Pinned resources' })
  ).toBeVisible();
  expect(
    screen.getByText('Your pinned resources will appear here.')
  ).toBeInTheDocument();
});

test('allows overriding the no-resources empty state content', async () => {
  await renderUnifiedResources({
    emptyState: {
      noResources: {
        title: 'Nothing here',
        description: 'Add your first resource.',
      },
    },
  });

  expect(screen.getByText('Nothing here')).toBeInTheDocument();
  expect(screen.getByText('Add your first resource.')).toBeInTheDocument();
  expect(screen.queryByText('No resources')).not.toBeInTheDocument();
});

test('allows replacing the no-resources empty state with a custom element', async () => {
  await renderUnifiedResources({
    emptyState: { noResources: <div>Custom empty panel</div> },
  });

  expect(screen.getByText('Custom empty panel')).toBeInTheDocument();
  expect(screen.queryByText('No resources')).not.toBeInTheDocument();
});

test.each([
  { params: { pinnedOnly: true }, title: 'Pinned resources' },
  { params: { search: 'foo' }, title: 'No resources match "foo"' },
  {
    params: { query: 'name == "foo"' },
    title: 'No resources match "name == "foo""',
  },
])(
  'does not apply the no-resources override when showing $title',
  async ({ params, title }) => {
    await renderUnifiedResources({
      params,
      emptyState: { noResources: <div>Custom empty panel</div> },
    });

    expect(screen.queryByText('Custom empty panel')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { name: title })).toBeVisible();
  }
);

test('allows overriding the no-matches empty state content', async () => {
  const noMatches = vi.fn().mockReturnValue({
    title: 'No labels matched',
    description: 'Try changing the labels.',
  });
  await renderUnifiedResources({
    params: { search: 'foo', pinnedOnly: true },
    emptyState: { noMatches },
  });

  expect(await screen.findByText('No labels matched')).toBeVisible();
  expect(screen.getByText('Try changing the labels.')).toBeInTheDocument();
  expect(noMatches).toHaveBeenCalledWith({ scope: 'pinned', search: 'foo' });
});

test('allows overriding the pinned empty state content', async () => {
  await renderUnifiedResources({
    params: { pinnedOnly: true },
    emptyState: {
      noPinned: {
        title: 'Nothing pinned',
        description: 'Pin a resource to see it here.',
      },
    },
  });

  expect(screen.getByText('Nothing pinned')).toBeInTheDocument();
  expect(
    screen.getByText('Pin a resource to see it here.')
  ).toBeInTheDocument();
  expect(
    screen.queryByText('Your pinned resources will appear here.')
  ).not.toBeInTheDocument();
});

test('allows overriding the pinned empty state with a custom element', async () => {
  await renderUnifiedResources({
    params: { pinnedOnly: true },
    emptyState: { noPinned: <div>Custom pinned panel</div> },
  });

  expect(screen.getByText('Custom pinned panel')).toBeInTheDocument();
});

test('allows overriding the no-matches empty state with static content', async () => {
  await renderUnifiedResources({
    params: { search: 'foo' },
    emptyState: {
      noMatches: () => ({
        title: 'Static no matches',
        description: 'Static description.',
      }),
    },
  });

  expect(await screen.findByText('Static no matches')).toBeVisible();
  expect(screen.getByText('Static description.')).toBeInTheDocument();
});

test('allows overriding the no-matches empty state with a custom element', async () => {
  await renderUnifiedResources({
    params: { search: 'foo' },
    emptyState: { noMatches: () => <div>Custom matches panel</div> },
  });

  expect(await screen.findByText('Custom matches panel')).toBeVisible();
});
