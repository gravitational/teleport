/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { fireEvent, render, screen, waitFor } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { InfoGuidePanelProvider } from 'teleport/Main/InfoGuideContext';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { yamlService } from 'teleport/services/yaml';

import { withDefaults } from './RoleEditor/StandardEditor/withDefaults';
import { Roles } from './Roles';
import { State } from './useRoles';

describe('Roles list', () => {
  const defaultState: State = {
    create: jest.fn(),
    fetch: jest.fn(),
    remove: jest.fn(),
    update: jest.fn(),
    rolesAcl: {
      read: true,
      remove: true,
      create: true,
      edit: true,
      list: true,
    },
  };

  beforeEach(() => {
    jest.spyOn(defaultState, 'fetch').mockResolvedValue({
      startKey: '',
      items: [
        {
          content: '',
          id: '1',
          kind: 'role',
          name: 'cool-role',
          description: 'coolest-role',
        },
      ],
    });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('button is enabled if user has create perms', async () => {
    const ctx = createTeleportContext();
    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...defaultState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('create_new_role_button')).toBeEnabled();
    });
  });

  test('displays disabled create button', async () => {
    const ctx = createTeleportContext();
    const testState = {
      ...defaultState,
      rolesAcl: {
        ...defaultState.rolesAcl,
        create: false,
      },
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...testState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByTestId('create_new_role_button')).toBeDisabled();
    });
  });

  test('all options available', async () => {
    const ctx = createTeleportContext();

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...defaultState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /options/i })
      ).toBeInTheDocument();
    });
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(2);
  });

  test('hides view/edit button if no access', async () => {
    const ctx = createTeleportContext();
    const testState = {
      ...defaultState,
      rolesAcl: {
        ...defaultState.rolesAcl,
        list: false,
      },
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...testState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /options/i })
      ).toBeInTheDocument();
    });
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(1);
    expect(
      menuItems.every(
        item =>
          item.textContent.includes('View') || item.textContent.includes('Edit')
      )
    ).not.toBe(true);
  });

  test('hides delete button if user does not have permission to delete', async () => {
    const ctx = createTeleportContext();
    const testState = {
      ...defaultState,
      rolesAcl: {
        ...defaultState.rolesAcl,
        remove: false,
      },
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...testState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: /options/i })
      ).toBeInTheDocument();
    });
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(1);
    expect(
      menuItems.every(item => item.textContent.includes('Delete'))
    ).not.toBe(true);
  });

  test('displays Options button if user has permission to list/read roles', async () => {
    const ctx = createTeleportContext();
    const testState = {
      ...defaultState,
      rolesAcl: {
        list: true,
        read: true,
        create: false,
        remove: false,
        edit: false,
      },
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...testState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('cool-role')).toBeInTheDocument();
    });
    const optionsButton = screen.getByRole('button', { name: /options/i });
    fireEvent.click(optionsButton);
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(1);
    expect(menuItems[0]).toHaveTextContent('View');
  });

  test('hides Options button if no permissions to view or delete', async () => {
    const ctx = createTeleportContext();
    const testState = {
      ...defaultState,
      rolesAcl: {
        ...defaultState.rolesAcl,
        remove: false,
        list: false,
      },
    };

    render(
      <MemoryRouter>
        <InfoGuidePanelProvider>
          <ContextProvider ctx={ctx}>
            <Roles {...testState} />
          </ContextProvider>
        </InfoGuidePanelProvider>
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('cool-role')).toBeInTheDocument();
    });
    const menuItems = screen.queryAllByRole('menuitem');
    expect(menuItems).toHaveLength(0);
  });
});

test('renders the role diff component', async () => {
  const ctx = createTeleportContext();
  const defaultState = (): State => ({
    create: jest.fn(),
    fetch: jest.fn().mockResolvedValue({
      startKey: '',
      items: [
        {
          content: '',
          id: '1',
          kind: 'role',
          name: 'cool-role',
          description: 'coolest-role',
        },
      ],
    }),
    remove: jest.fn(),
    update: jest.fn(),
    rolesAcl: {
      read: true,
      remove: true,
      create: true,
      edit: true,
      list: true,
    },
  });
  jest.spyOn(yamlService, 'parse').mockImplementation(async () => {
    return withDefaults({});
  });
  const roleDiffElement = <div>i am rendered</div>;

  render(
    <MemoryRouter>
      <InfoGuidePanelProvider>
        <ContextProvider ctx={ctx}>
          <Roles
            {...defaultState()}
            roleDiffProps={{
              roleDiffElement,
              updateRoleDiff: () => null,
              roleDiffAttempt: {
                status: 'error',
                statusText: 'there is an error here',
                data: null,
                error: null,
              },
            }}
          />
        </ContextProvider>
      </InfoGuidePanelProvider>
    </MemoryRouter>
  );
  await openEditor();
  expect(screen.getByText('i am rendered')).toBeInTheDocument();
  expect(await screen.findByText('there is an error here')).toBeInTheDocument();
});

async function openEditor() {
  await waitFor(() => {
    expect(screen.getByText('cool-role')).toBeInTheDocument();
  });
  const optionsButton = screen.getByRole('button', { name: /options/i });
  fireEvent.click(optionsButton);
  const menuItems = screen.queryAllByRole('menuitem');
  fireEvent.click(menuItems[0]);
}
