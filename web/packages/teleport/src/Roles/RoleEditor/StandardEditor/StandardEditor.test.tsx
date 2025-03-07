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

import { within } from '@testing-library/react';
import { UserEvent } from '@testing-library/user-event';
import { produce } from 'immer';
import selectEvent from 'react-select-event';

import { render, screen, userEvent } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { createTeleportContext } from 'teleport/mocks/contexts';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import TeleportContextProvider from 'teleport/TeleportContextProvider';

import { StandardEditor, StandardEditorProps } from './StandardEditor';
import { newRole } from './standardmodel';
import { useStandardModel } from './useStandardModel';

const TestStandardEditor = (props: Partial<StandardEditorProps>) => {
  const ctx = createTeleportContext();
  const [model, dispatch] = useStandardModel(props.originalRole?.object);
  return (
    <TeleportContextProvider ctx={ctx}>
      <Validation>
        <StandardEditor
          standardEditorModel={model}
          isProcessing={false}
          dispatch={dispatch}
          {...props}
        />
      </Validation>
    </TeleportContextProvider>
  );
};

let user: UserEvent;

beforeEach(() => {
  user = userEvent.setup();
});

test('adding and removing sections', async () => {
  render(<TestStandardEditor originalRole={newRoleWithYaml(newRole())} />);
  expect(getAllSectionNames()).toEqual(['Role Metadata']);
  await user.click(getTabByName('Resources'));
  expect(getAllSectionNames()).toEqual([]);

  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Servers',
    'Applications',
    'Databases',
    'Windows Desktops',
    'GitHub Organizations',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  expect(getAllSectionNames()).toEqual(['Servers']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Applications',
    'Databases',
    'Windows Desktops',
    'GitHub Organizations',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes' }));
  expect(getAllSectionNames()).toEqual(['Servers', 'Kubernetes']);

  await user.click(
    within(getSectionByName('Servers')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Kubernetes']);

  await user.click(
    within(getSectionByName('Kubernetes')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual([]);
});

test('collapsed sections still apply validation', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name *'));
  // Collapse the section.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).not.toHaveBeenCalled();

  // Expand the section, make it valid.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.type(screen.getByLabelText('Role Name *'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).toHaveBeenCalled();
});

test('invisible tabs still apply validation', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name *'));
  // Switch to a different tab.
  await user.click(getTabByName('Resources'));
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).not.toHaveBeenCalled();

  // Switch back, make it valid.
  await user.click(getTabByName('Invalid data Overview'));
  await user.type(screen.getByLabelText('Role Name *'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).toHaveBeenCalled();
});

test('edits metadata', async () => {
  let role: Role | undefined;
  const onSave = (r: Role) => (role = r);
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );
  await user.type(screen.getByLabelText('Description'), 'foo');
  await selectEvent.select(screen.getByLabelText('Version'), 'v6');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(role.metadata.description).toBe('foo');
  expect(role.version).toBe('v6');
});

test('edits resource access', async () => {
  let role: Role | undefined;
  const onSave = (r: Role) => (role = r);
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  await selectEvent.create(screen.getByLabelText('Logins'), 'ec2-user', {
    createOptionText: 'Login: ec2-user',
  });
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(role.spec.allow.logins).toEqual(['{{internal.logins}}', 'ec2-user']);
});

test('triggers v6 validation for Kubernetes resources', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );
  await selectEvent.select(screen.getByLabelText('Version'), 'v6');
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes' }));
  await user.click(screen.getByRole('button', { name: 'Add a Resource' }));
  await selectEvent.select(screen.getByLabelText('Kind'), 'Job');

  // Adding a second resource to make sure that we don't run into attempting to
  // modify an immer-frozen object. This might happen if the reducer tried to
  // modify resources that were already there.
  await user.click(
    screen.getByRole('button', { name: 'Add Another Resource' })
  );
  await selectEvent.select(screen.getAllByLabelText('Kind')[1], 'Pod');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));

  // Validation should have failed on a Job resource and role v6.
  expect(
    screen.getByText('Only pods are allowed for role version v6')
  ).toBeVisible();
  expect(onSave).not.toHaveBeenCalled();

  // Back to v7, try again
  await user.click(getTabByName('Overview'));
  await selectEvent.select(screen.getByLabelText('Version'), 'v7');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).toHaveBeenCalled();
});

test('creating a new role', async () => {
  async function forwardToTab(name: string) {
    expect(
      screen.queryByRole('button', { name: 'Create Role' })
    ).not.toBeInTheDocument();
    const tab = getTabByName(name);
    expect(tab).toBeDisabled();
    await user.click(screen.getByRole('button', { name: `Next: ${name}` }));
    expect(tab).toHaveAttribute('aria-selected', 'true');
  }

  const onSave = jest.fn();
  render(<TestStandardEditor onSave={onSave} />);
  await user.type(screen.getByLabelText('Description'), 'foo');
  await forwardToTab('Resources');
  await forwardToTab('Access Rules');
  await forwardToTab('Options');
  expect(onSave).not.toHaveBeenCalled();

  // By now, all the tabs should be enabled.
  expect(getTabByName('Overview')).toBeEnabled();
  expect(getTabByName('Resources')).toBeEnabled();
  expect(getTabByName('Access Rules')).toBeEnabled();
  expect(getTabByName('Options')).toBeEnabled();

  // Allow free navigation.
  await user.click(getTabByName('Resources'));
  expect(getTabByName('Resources')).toHaveAttribute('aria-selected', 'true');
  await user.click(getTabByName('Options'));
  expect(getTabByName('Options')).toHaveAttribute('aria-selected', 'true');

  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).toHaveBeenCalledWith(
    produce(newRole(), r => {
      r.metadata.description = 'foo';
    })
  );
});

test('tab-level validation when creating a new role', async () => {
  render(<TestStandardEditor />);
  // Break the validation and attempt switching tabs.
  await user.clear(screen.getByLabelText('Role Name *'));
  await user.click(screen.getByRole('button', { name: 'Next: Resources' }));
  expect(getTabByName('Resources')).toHaveAttribute('aria-selected', 'false');
  // Fix the field value and retry.
  expect(screen.getByLabelText('Role Name *')).toHaveAccessibleDescription(
    'Role name is required'
  );
  await user.type(screen.getByLabelText('Role Name *'), 'some-role');
  await user.click(screen.getByRole('button', { name: 'Next: Resources' }));
  expect(getTabByName('Resources')).toHaveAttribute('aria-selected', 'true');

  // Break the validation and attempt switching tabs.
  await user.click(
    screen.getByRole('button', { name: 'Add New Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  // The form should not be validating until we try to switch to the next tab.
  expect(screen.getByPlaceholderText('label key')).toHaveAccessibleDescription(
    ''
  );
  await user.click(screen.getByRole('button', { name: 'Next: Access Rules' }));
  expect(getTabByName('Access Rules')).toHaveAttribute(
    'aria-selected',
    'false'
  );
  // Fix the field value and retry.
  await user.type(screen.getByPlaceholderText('label key'), 'foo');
  await user.type(screen.getByPlaceholderText('label value'), 'bar');
  await user.click(screen.getByRole('button', { name: 'Next: Access Rules' }));
  expect(getTabByName('Access Rules')).toHaveAttribute('aria-selected', 'true');
});

const getAllMenuItemNames = () =>
  screen.queryAllByRole('menuitem').map(m => m.textContent);

const getAllSectionNames = () =>
  screen.queryAllByRole('heading', { level: 3 }).map(m => m.textContent);

const getTabByName = (name: string) => screen.getByRole('tab', { name });

const getSectionByName = (name: string) =>
  // There's no better way to do it, unfortunately.
  // eslint-disable-next-line testing-library/no-node-access
  screen.getByRole('heading', { level: 3, name }).closest('details');

const newRoleWithYaml = (role: Role): RoleWithYaml => ({
  object: role,
  yaml: '{}', // Irrelevant in the standard editor context.
});
