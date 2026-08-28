import { MemoryRouter } from 'react-router';
import selectEvent from 'react-select-event';

import {
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { ContextProvider } from 'teleport/index';
import { IntegrationStatusCode, Plugin } from 'teleport/services/integrations';
import userService from 'teleport/services/user';

import { AccessListOwnersSource, Filters } from '../types';
import { emptyFilter, filterCollection } from './constants';
import { SyncSettings } from './SyncSettings';

beforeEach(() => {
  jest.spyOn(userService, 'fetchUsersV2').mockResolvedValue({
    items: [
      {
        name: 'alice',
        roles: [],
        displayPrimary: 'Alice Adams',
        displaySecondary: 'alice@example.com',
      },
      { name: 'bob', roles: [] },
      { name: 'carol', roles: [] },
    ],
    startKey: '',
  });
});

afterEach(() => {
  jest.clearAllMocks();
});

test('edit settings', async () => {
  const onSave = jest.fn();
  renderSyncSettings(undefined /** plugin */, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Sync Settings')).toBeInTheDocument();
  });

  const deltaInterval = screen.getByLabelText('Delta Sync Interval');
  await userEvent.clear(deltaInterval); // clear default 0s
  await userEvent.type(deltaInterval, '2m');

  const fullInterval = screen.getByLabelText('Full Sync Interval');
  await userEvent.clear(fullInterval); // clear default 0s
  await userEvent.type(fullInterval, '1h');

  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  const importAll = screen.getByTestId('toggle');
  expect(importAll).toBeChecked();
  //  Toggle off to configure filters
  fireEvent.click(importAll);

  const filters: Filters = {
    id: ['g1', 'g2'],
    nameRegex: ['admin-*'],
    excludeId: ['g2'],
    excludeNameRegex: ['hr*'],
  };
  setFilterInputs(filters);

  // select first user option
  const users = screen.getByText(/type a username/i);
  fireEvent.keyDown(users, { key: 'ArrowDown' });
  expect(await screen.findByText('Alice Adams')).toBeVisible();
  expect(screen.getByText('alice@example.com')).toBeVisible();
  expect(screen.getByText('alice')).toBeVisible();
  fireEvent.keyDown(users, { key: 'Enter' });

  expect(screen.getByText('Alice Adams')).toBeVisible();
  expect(screen.getByText('alice')).toBeVisible();

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    filters,
    ['alice'],
    AccessListOwnersSource.Plugin,
    { delta: '2m', full: '1h' }
  );
});

test('import all toogle on', async () => {
  const onSave = jest.fn();
  const filters: Filters = {
    id: ['g1', 'g2'],
    nameRegex: ['admin-*'],
    excludeId: ['g2'],
    excludeNameRegex: ['hr*'],
  };
  const plugin: Plugin = {
    kind: 'entra-id',
    name: 'entra-id-default',
    resourceType: 'plugin',
    statusCode: IntegrationStatusCode.Running,
    spec: {
      defaultOwners: ['alice', 'bob'],
      groupFilters: filters,
    },
  };
  renderSyncSettings(plugin, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Sync Settings')).toBeInTheDocument();
  });

  // Group filters
  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  const importAll = screen.getByTestId('toggle');
  expect(importAll).not.toBeChecked();

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    plugin.spec.groupFilters,
    plugin.spec.defaultOwners,
    AccessListOwnersSource.Plugin,
    { delta: '0s', full: '0s' }
  );

  onSave.mockReset();

  // Toggle off to configure filters
  fireEvent.click(importAll);

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    emptyFilter,
    plugin.spec.defaultOwners,
    AccessListOwnersSource.Plugin,
    { delta: '0s', full: '0s' }
  );
});

test('default owner validation', async () => {
  renderSyncSettings();

  await waitFor(() => {
    expect(screen.getByText('Edit Sync Settings')).toBeInTheDocument();
  });

  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  const importAll = screen.getByTestId('toggle');
  expect(importAll).toBeChecked();

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(
    screen.getByText('At least 1 default owner is required')
  ).toBeInTheDocument();
});

test('prefill values from plugin spec', async () => {
  const onSave = jest.fn();
  const filters: Filters = {
    id: ['g1', 'g2'],
    nameRegex: ['admin-*'],
    excludeId: ['g2'],
    excludeNameRegex: ['hr*'],
  };
  const plugin: Plugin = {
    kind: 'entra-id',
    name: 'entra-id-default',
    resourceType: 'plugin',
    statusCode: IntegrationStatusCode.Running,
    spec: {
      defaultOwners: ['alice', 'bob'],
      groupFilters: filters,
      syncIntervals: { delta: '0s', full: '1h' },
    },
  };
  renderSyncSettings(plugin, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Sync Settings')).toBeInTheDocument();
  });

  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  expect(screen.getByTestId('toggle')).not.toBeChecked();
  expect(screen.getByText(`Access Lists owner(s)`)).toBeInTheDocument();
  expect(screen.getByText('alice')).toBeVisible();
  expect(screen.getByText('bob')).toBeVisible();
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    plugin.spec.groupFilters,
    plugin.spec.defaultOwners,
    AccessListOwnersSource.Plugin,
    { delta: '0s', full: '1h' }
  );
});

test('renders and submits a manually entered owner by username', async () => {
  const onSave = jest.fn();
  renderSyncSettings(undefined, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Sync Settings')).toBeInTheDocument();
  });

  const users = screen.getByRole('combobox');
  await selectEvent.create(users, 'external-owner');

  expect(screen.getByText('external-owner')).toBeVisible();
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    emptyFilter,
    ['external-owner'],
    AccessListOwnersSource.Plugin,
    { delta: '0s', full: '0s' }
  );
});

function renderSyncSettings(
  plugin?: Plugin,
  onSave?: (filters: Filters, owners: string[], ownersSource: string) => void
) {
  render(
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextE()}>
        <SyncSettings plugin={plugin} onSave={onSave} disabled={false} />
      </ContextProvider>
    </MemoryRouter>
  );
}

function setFilterInputs(filter: Filters) {
  const includeId = screen.getByLabelText(filterCollection[0].label);
  expect(includeId).toBeInTheDocument();
  fireEvent.change(includeId, { target: { value: filter.id[0] } });
  fireEvent.keyDown(includeId, { key: 'Enter' });

  fireEvent.change(includeId, { target: { value: filter.id[1] } });
  fireEvent.keyDown(includeId, { key: 'Enter' });

  const includeNameRegex = screen.getByLabelText(filterCollection[1].label);
  expect(includeNameRegex).toBeInTheDocument();
  fireEvent.change(includeNameRegex, { target: { value: filter.nameRegex } });
  fireEvent.keyDown(includeNameRegex, { key: 'Enter' });

  const excludeId = screen.getByLabelText(filterCollection[2].label);
  expect(excludeId).toBeInTheDocument();
  fireEvent.change(excludeId, { target: { value: filter.excludeId } });
  fireEvent.keyDown(excludeId, { key: 'Enter' });

  const excludeNameRegex = screen.getByLabelText(filterCollection[3].label);
  expect(excludeNameRegex).toBeInTheDocument();
  fireEvent.change(excludeNameRegex, {
    target: { value: filter.excludeNameRegex },
  });
  fireEvent.keyDown(excludeNameRegex, { key: 'Enter' });
}
