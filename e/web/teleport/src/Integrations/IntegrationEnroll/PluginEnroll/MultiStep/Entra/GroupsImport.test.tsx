import { MemoryRouter } from 'react-router';

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

import {
  EditGroupsImport,
  emptyFilter,
  filterCollection,
  hasZeroFilters,
} from './GroupsImport';
import { Filters } from './types';

beforeEach(() => {
  jest.spyOn(userService, 'fetchUsersV2').mockResolvedValue({
    items: [
      { name: 'alice', roles: [] },
      { name: 'bob', roles: [] },
      { name: 'carol', roles: [] },
    ],
    startKey: '',
  });
});

afterEach(() => {
  jest.clearAllMocks();
});

test('edit page default', async () => {
  const onSave = jest.fn();
  renderGroupsImport(undefined /** plugin */, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Group Import Settings')).toBeInTheDocument();
  });

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
  fireEvent.keyDown(users, { key: 'Enter' });

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(filters, ['alice']);
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
  renderGroupsImport(plugin, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Group Import Settings')).toBeInTheDocument();
  });

  // Group filters
  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  const importAll = screen.getByTestId('toggle');
  expect(importAll).not.toBeChecked();

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    plugin.spec.groupFilters,
    plugin.spec.defaultOwners
  );

  onSave.mockReset();

  // Toggle off to configure filters
  fireEvent.click(importAll);

  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(emptyFilter, plugin.spec.defaultOwners);
});

test('default owner validation', async () => {
  renderGroupsImport();

  await waitFor(() => {
    expect(screen.getByText('Edit Group Import Settings')).toBeInTheDocument();
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
    },
  };
  renderGroupsImport(plugin, onSave);

  await waitFor(() => {
    expect(screen.getByText('Edit Group Import Settings')).toBeInTheDocument();
  });

  expect(screen.getByText('Group Filters')).toBeInTheDocument();
  expect(screen.getByTestId('toggle')).not.toBeChecked();
  expect(screen.getByText(`Default Access Lists owner(s)`)).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: /save/i }));

  expect(onSave).toHaveBeenCalledWith(
    plugin.spec.groupFilters,
    plugin.spec.defaultOwners
  );
});

function renderGroupsImport(
  plugin?: Plugin,
  onSave?: (filters: Filters, owners: string[]) => void
) {
  render(
    <MemoryRouter>
      <ContextProvider ctx={createTeleportContextE()}>
        <EditGroupsImport plugin={plugin} onSave={onSave} disabled={false} />
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

describe('hasZeroFilters', () => {
  const predicates: {
    name: string;
    filters: any;
    expected: boolean;
  }[] = [
    {
      name: 'with all filters',
      filters: {
        id: ['g1', 'g2'],
        nameRegex: ['admin-*'],
        excludeId: ['g2'],
        excludeNameRegex: ['hr*'],
      },
      expected: false,
    },
    {
      name: 'partial filters',
      filters: {
        id: ['g1', 'g2'],
        nameRegex: [],
        excludeId: ['g2'],
        excludeNameRegex: ['hr*'],
      },
      expected: false,
    },
    {
      name: 'empty filters',
      filters: {
        id: [],
        nameRegex: [],
        excludeId: [],
        excludeNameRegex: [],
      },
      expected: true,
    },
    {
      name: 'empty',
      filters: {},
      expected: true,
    },
  ];
  test.each(predicates)('$name', ({ filters, expected }) => {
    expect(hasZeroFilters(filters)).toBe(expected);
  });
});
