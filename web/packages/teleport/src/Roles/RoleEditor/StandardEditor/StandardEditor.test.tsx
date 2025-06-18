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
import { ApiError } from 'teleport/services/api/parseError';
import ResourceService, {
  Role,
  RoleWithYaml,
} from 'teleport/services/resources';
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
  jest
    .spyOn(ResourceService.prototype, 'fetchRole')
    .mockImplementation(async name => {
      // Make sure that validation never fails because of a role name collision.
      throw new ApiError({
        message: `role ${name} is not found`,
        response: { status: 404 } as Response,
      });
    });
});

afterEach(() => {
  jest.restoreAllMocks();
});

test('adding and removing sections', async () => {
  render(<TestStandardEditor originalRole={newRoleWithYaml(newRole())} />);
  await user.click(getTabByName('Resources'));
  expect(getAllSectionNames()).toEqual([]);

  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes Access',
    'SSH Server Access',
    'Application Access',
    'Database Access',
    'Windows Desktop Access',
    'GitHub Organization Access',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'SSH Server Access' }));
  expect(getAllSectionNames()).toEqual(['SSH Server Access']);

  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes Access',
    'Application Access',
    'Database Access',
    'Windows Desktop Access',
    'GitHub Organization Access',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes Access' }));
  expect(getAllSectionNames()).toEqual([
    'SSH Server Access',
    'Kubernetes Access',
  ]);

  await user.click(
    within(getSectionByName('SSH Server Access')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Kubernetes Access']);

  await user.click(
    within(getSectionByName('Kubernetes Access')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual([]);
});

test('invisible tabs still apply validation', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );

  // Cause a validation error by adding a label with an empty key.
  await user.type(screen.getByPlaceholderText('label value'), 'bar');

  // Switch to a different tab.
  await user.click(getTabByName('Resources'));
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).not.toHaveBeenCalled();

  // Switch back, make it valid.
  await user.click(getTabByName('Overview Invalid data'));
  await user.type(screen.getByPlaceholderText('label key'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).toHaveBeenCalled();
});

test('hidden validation errors should not propagate to tab headings', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );

  // Cause a validation error by adding a label with an empty key.
  await user.type(screen.getByPlaceholderText('label value'), 'bar');
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).not.toHaveBeenCalled();

  // Switch to the Resources tab. Add a new section and make it invalid.
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'SSH Server Access' }));
  await user.type(
    within(getSectionByName('SSH Server Access')).getByPlaceholderText(
      'label value'
    ),
    'some-value'
  );

  // Switch to the Admin Rules tab. Add a new section (it's invalid by
  // default).
  await user.click(getTabByName('Admin Rules'));
  await user.click(screen.getByRole('button', { name: 'Add New' }));

  // Switch back. The newly invalid tabs should not bear the invalid indicator,
  // as the section has its validation errors hidden.
  await user.click(getTabByName('Overview Invalid data'));
  expect(getTabByName('Resources')).toBeInTheDocument();
  expect(getTabByName('Admin Rules')).toBeInTheDocument();

  // Attempt to save, causing global validation. Now the invalid tabs should be
  // marked as invalid.
  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(getTabByName('Resources Invalid data')).toBeInTheDocument();
  expect(getTabByName('Admin Rules Invalid data')).toBeInTheDocument();
  expect(onSave).not.toHaveBeenCalled();
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
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'SSH Server Access' }));
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

  // Select v7 so we can set a different value.
  await selectEvent.select(screen.getByLabelText('Version'), 'v7');
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes Access' }));
  await user.click(
    screen.getByRole('button', { name: 'Add a Kubernetes Resource' })
  );
  await selectEvent.select(screen.getByLabelText('Kind'), 'Job');

  // Back to v6 to check validation.
  await user.click(getTabByName('Overview'));
  await selectEvent.select(screen.getByLabelText('Version'), 'v6');
  await user.click(getTabByName('Resources'));

  // Adding a second resource to make sure that we don't run into attempting to
  // modify an immer-frozen object. This might happen if the reducer tried to
  // modify resources that were already there.
  await user.click(
    screen.getByRole('button', { name: 'Add Another Kubernetes Resource' })
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
}, 10000);

test('triggers v7 validation for Kubernetes resources', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );

  // Select v8 so we can set a different value.
  await selectEvent.select(screen.getByLabelText('Version'), 'v8');
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes Access' }));
  await user.click(
    screen.getByRole('button', { name: 'Add a Kubernetes Resource' })
  );
  await selectEvent.select(screen.getByLabelText('Kind (plural)'), 'jobs');

  // Go to v7 to check validation.
  await user.click(getTabByName('Overview'));
  await selectEvent.select(screen.getByLabelText('Version'), 'v7');
  await user.click(getTabByName('Resources'));

  // Adding a second resource to make sure that we don't run into attempting to
  // modify an immer-frozen object. This might happen if the reducer tried to
  // modify resources that were already there.
  await user.click(
    screen.getByRole('button', { name: 'Add Another Kubernetes Resource' })
  );

  await user.click(screen.getByRole('button', { name: 'Save Changes' }));

  // Validation should have failed on a jobs resource and role v7.
  expect(
    screen.getByText(
      'Only core predefined kinds are allowed for role version v7'
    )
  ).toBeVisible();
  // Validation should have failed on a api groups being set and role v7.
  expect(
    screen.getByText('API Group not supported for role version v7.')
  ).toBeVisible();
  expect(onSave).not.toHaveBeenCalled();

  // Back to v8, try again
  await user.click(getTabByName('Overview'));
  await selectEvent.select(screen.getByLabelText('Version'), 'v8');

  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).not.toHaveBeenCalled();
}, 10000);

test('triggers v8 validation for Kubernetes resources', async () => {
  const onSave = jest.fn();
  render(
    <TestStandardEditor
      originalRole={newRoleWithYaml(newRole())}
      onSave={onSave}
    />
  );

  // Select v7 so we can set a known value.
  await selectEvent.select(screen.getByLabelText('Version'), 'v7');
  await user.click(getTabByName('Resources'));
  await user.click(
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes Access' }));
  await user.click(
    screen.getByRole('button', { name: 'Add a Kubernetes Resource' })
  );
  await selectEvent.select(screen.getByLabelText('Kind'), 'Job');

  // Go to v8 to check validation.
  await user.click(getTabByName('Overview'));
  await selectEvent.select(screen.getByLabelText('Version'), 'v8');
  await user.click(getTabByName('Resources'));

  // Adding a second resource to make sure that we don't run into attempting to
  // modify an immer-frozen object. This might happen if the reducer tried to
  // modify resources that were already there.
  await user.click(
    screen.getByRole('button', { name: 'Add Another Kubernetes Resource' })
  );

  // Set the api group for the first resource.
  await user.type(screen.getAllByLabelText('API Group *')[0], '*');
  // Clear the api group for the second resource.
  await user.clear(screen.getAllByLabelText('API Group *')[1]);

  await user.click(screen.getByRole('button', { name: 'Save Changes' }));

  // Validation should have failed on a v7 kind and role v8.
  expect(
    screen.getByText('Kind must use k8s plural name. Did you mean "jobs"?')
  ).toBeVisible();

  // Validation should have failed on a api group being missing and role v8.
  expect(
    screen.getByText('API Group required. Use "*" for any group.')
  ).toBeVisible();
  expect(onSave).not.toHaveBeenCalled();

  // Fix the validation errors and try again.
  await selectEvent.select(
    screen.getAllByLabelText('Kind (plural)')[0],
    'jobs'
  );
  await user.type(screen.getAllByLabelText('API Group *')[1], '*');

  await user.click(screen.getByRole('button', { name: 'Save Changes' }));
  expect(onSave).toHaveBeenCalled();
}, 10000);

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
  await forwardToTab('Admin Rules');
  await forwardToTab('Options');
  expect(onSave).not.toHaveBeenCalled();

  // By now, all the tabs should be enabled.
  expect(getTabByName('Overview')).toBeEnabled();
  expect(getTabByName('Resources')).toBeEnabled();
  expect(getTabByName('Admin Rules')).toBeEnabled();
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
    screen.getByRole('button', { name: 'Add Teleport Resource Access' })
  );
  await user.click(screen.getByRole('menuitem', { name: 'SSH Server Access' }));
  await user.type(
    within(getSectionByName('SSH Server Access')).getByPlaceholderText(
      'label value'
    ),
    'bar'
  );
  // The form should not be validating until we try to switch to the next tab.
  expect(
    within(getSectionByName('SSH Server Access')).getByPlaceholderText(
      'label key'
    )
  ).toHaveAccessibleDescription('');
  await user.click(screen.getByRole('button', { name: 'Next: Admin Rules' }));
  expect(getTabByName('Admin Rules')).toHaveAttribute('aria-selected', 'false');
  // Fix the field value and retry.
  await user.type(
    within(getSectionByName('SSH Server Access')).getByPlaceholderText(
      'label key'
    ),
    'foo'
  );
  await user.click(screen.getByRole('button', { name: 'Next: Admin Rules' }));
  expect(getTabByName('Admin Rules')).toHaveAttribute('aria-selected', 'true');
});

const getAllMenuItemNames = () =>
  screen.queryAllByRole('menuitem').map(m => m.textContent);

const getAllSectionNames = () =>
  screen.queryAllByRole('heading', { level: 3 }).map(m => m.textContent.trim());

const getTabByName = (name: string) => screen.getByRole('tab', { name });

const getSectionByName = (name: string) =>
  // There's no better way to do it, unfortunately.
  // eslint-disable-next-line testing-library/no-node-access
  screen.getByRole('heading', { level: 3, name }).closest('details');

const newRoleWithYaml = (role: Role): RoleWithYaml => ({
  object: role,
  yaml: '{}', // Irrelevant in the standard editor context.
});
