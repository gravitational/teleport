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

import { render, screen, userEvent } from 'design/utils/testing';
import React, { useState } from 'react';

import { within } from '@testing-library/react';
import Validation from 'shared/components/Validation';
import selectEvent from 'react-select-event';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  AccessSpec,
  AppAccessSpec,
  DatabaseAccessSpec,
  KubernetesAccessSpec,
  newAccessSpec,
  newRole,
  roleToRoleEditorModel,
  ServerAccessSpec,
  StandardEditorModel,
  WindowsDesktopAccessSpec,
} from './standardmodel';
import {
  AppAccessSpecSection,
  DatabaseAccessSpecSection,
  KubernetesAccessSpecSection,
  SectionProps,
  ServerAccessSpecSection,
  StandardEditor,
  StandardEditorProps,
  WindowsDesktopAccessSpecSection,
} from './StandardEditor';

const TestStandardEditor = (props: Partial<StandardEditorProps>) => {
  const ctx = createTeleportContext();
  const [model, setModel] = useState<StandardEditorModel>({
    roleModel: roleToRoleEditorModel(newRole()),
    isDirty: true,
  });
  return (
    <TeleportContextProvider ctx={ctx}>
      <StandardEditor
        originalRole={null}
        standardEditorModel={model}
        isProcessing={false}
        onChange={setModel}
        {...props}
      />
    </TeleportContextProvider>
  );
};

test('adding and removing sections', async () => {
  const user = userEvent.setup();
  render(<TestStandardEditor />);
  expect(getAllSectionNames()).toEqual(['Role Metadata']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Specifications' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Servers',
    'Applications',
    'Databases',
    'Windows Desktops',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Servers' }));
  expect(getAllSectionNames()).toEqual(['Role Metadata', 'Servers']);

  await user.click(
    screen.getByRole('button', { name: 'Add New Specifications' })
  );
  expect(getAllMenuItemNames()).toEqual([
    'Kubernetes',
    'Applications',
    'Databases',
    'Windows Desktops',
  ]);

  await user.click(screen.getByRole('menuitem', { name: 'Kubernetes' }));
  expect(getAllSectionNames()).toEqual([
    'Role Metadata',
    'Servers',
    'Kubernetes',
  ]);

  await user.click(
    within(getSectionByName('Servers')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Role Metadata', 'Kubernetes']);

  await user.click(
    within(getSectionByName('Kubernetes')).getByRole('button', {
      name: 'Remove section',
    })
  );
  expect(getAllSectionNames()).toEqual(['Role Metadata']);
});

test('collapsed sections still apply validation', async () => {
  const user = userEvent.setup();
  const onSave = jest.fn();
  render(<TestStandardEditor onSave={onSave} />);
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name'));
  // Collapse the section.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).not.toHaveBeenCalled();

  // Expand the section, make it valid.
  await user.click(screen.getByRole('heading', { name: 'Role Metadata' }));
  await user.type(screen.getByLabelText('Role Name'), 'foo');
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).toHaveBeenCalled();
});

const getAllMenuItemNames = () =>
  screen.queryAllByRole('menuitem').map(m => m.textContent);

const getAllSectionNames = () =>
  screen.queryAllByRole('heading', { level: 3 }).map(m => m.textContent);

const getSectionByName = (name: string) =>
  // There's no better way to do it, unfortunately.
  // eslint-disable-next-line testing-library/no-node-access
  screen.getByRole('heading', { level: 3, name }).closest('details');

const StatefulSection = <S extends AccessSpec>({
  defaultValue,
  component: Component,
  onChange,
}: {
  defaultValue: S;
  component: React.ComponentType<SectionProps<S>>;
  onChange(spec: S): void;
}) => {
  const [model, setModel] = useState<S>(defaultValue);
  return (
    <Validation>
      <Component
        value={model}
        isProcessing={false}
        onChange={spec => {
          setModel(spec);
          onChange(spec);
        }}
      />
    </Validation>
  );
};

test('ServerAccessSpecSection', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(
    <StatefulSection<ServerAccessSpec>
      component={ServerAccessSpecSection}
      defaultValue={newAccessSpec('node')}
      onChange={onChange}
    />
  );
  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  await user.type(screen.getByPlaceholderText('label key'), 'some-key');
  await user.type(screen.getByPlaceholderText('label value'), 'some-value');
  await selectEvent.create(screen.getByLabelText('Logins'), 'root', {
    createOptionText: 'Login: root',
  });
  await selectEvent.create(screen.getByLabelText('Logins'), 'some-user', {
    createOptionText: 'Login: some-user',
  });

  expect(onChange).toHaveBeenLastCalledWith({
    kind: 'node',
    labels: [{ name: 'some-key', value: 'some-value' }],
    logins: [
      expect.objectContaining({ label: 'root', value: 'root' }),
      expect.objectContaining({ label: 'some-user', value: 'some-user' }),
    ],
  } as ServerAccessSpec);
});

describe('KubernetesAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    render(
      <StatefulSection<KubernetesAccessSpec>
        component={KubernetesAccessSpecSection}
        defaultValue={newAccessSpec('kube_cluster')}
        onChange={onChange}
      />
    );
    return { user: userEvent.setup(), onChange };
  };

  test('editing the spec', async () => {
    const { user, onChange } = setup();

    await selectEvent.create(screen.getByLabelText('Groups'), 'group1', {
      createOptionText: 'Group: group1',
    });
    await selectEvent.create(screen.getByLabelText('Groups'), 'group2', {
      createOptionText: 'Group: group2',
    });

    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.type(screen.getByPlaceholderText('label key'), 'some-key');
    await user.type(screen.getByPlaceholderText('label value'), 'some-value');

    await user.click(screen.getByRole('button', { name: 'Add a Resource' }));
    expect(
      reactSelectValueContainer(screen.getByLabelText('Kind'))
    ).toHaveTextContent('Any kind');
    expect(screen.getByLabelText('Name')).toHaveValue('*');
    expect(screen.getByLabelText('Namespace')).toHaveValue('*');
    await selectEvent.select(screen.getByLabelText('Kind'), 'Job');
    await user.clear(screen.getByLabelText('Name'));
    await user.type(screen.getByLabelText('Name'), 'job-name');
    await user.clear(screen.getByLabelText('Namespace'));
    await user.type(screen.getByLabelText('Namespace'), 'job-namespace');
    await selectEvent.select(screen.getByLabelText('Verbs'), [
      'create',
      'delete',
    ]);

    expect(onChange).toHaveBeenLastCalledWith({
      kind: 'kube_cluster',
      groups: [
        expect.objectContaining({ value: 'group1' }),
        expect.objectContaining({ value: 'group2' }),
      ],
      labels: [{ name: 'some-key', value: 'some-value' }],
      resources: [
        {
          id: expect.any(String),
          kind: expect.objectContaining({ value: 'job' }),
          name: 'job-name',
          namespace: 'job-namespace',
          verbs: [
            expect.objectContaining({ value: 'create' }),
            expect.objectContaining({ value: 'delete' }),
          ],
        },
      ],
    } as KubernetesAccessSpec);
  });

  test('adding and removing resources', async () => {
    const { user, onChange } = setup();

    await user.click(screen.getByRole('button', { name: 'Add a Resource' }));
    await user.clear(screen.getByLabelText('Name'));
    await user.type(screen.getByLabelText('Name'), 'res1');
    await user.click(
      screen.getByRole('button', { name: 'Add Another Resource' })
    );
    await user.clear(screen.getAllByLabelText('Name')[1]);
    await user.type(screen.getAllByLabelText('Name')[1], 'res2');
    await user.click(
      screen.getByRole('button', { name: 'Add Another Resource' })
    );
    await user.clear(screen.getAllByLabelText('Name')[2]);
    await user.type(screen.getAllByLabelText('Name')[2], 'res3');
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        resources: [
          expect.objectContaining({ name: 'res1' }),
          expect.objectContaining({ name: 'res2' }),
          expect.objectContaining({ name: 'res3' }),
        ],
      })
    );

    await user.click(
      screen.getAllByRole('button', { name: 'Remove resource' })[1]
    );
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        resources: [
          expect.objectContaining({ name: 'res1' }),
          expect.objectContaining({ name: 'res3' }),
        ],
      })
    );
    await user.click(
      screen.getAllByRole('button', { name: 'Remove resource' })[0]
    );
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        resources: [expect.objectContaining({ name: 'res3' })],
      })
    );
    await user.click(
      screen.getAllByRole('button', { name: 'Remove resource' })[0]
    );
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ resources: [] })
    );
  });
});

test('AppAccessSpecSection', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(
    <StatefulSection<AppAccessSpec>
      component={AppAccessSpecSection}
      defaultValue={newAccessSpec('app')}
      onChange={onChange}
    />
  );

  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  await user.type(screen.getByPlaceholderText('label key'), 'env');
  await user.type(screen.getByPlaceholderText('label value'), 'prod');
  await user.type(
    within(screen.getByRole('group', { name: 'AWS Role ARNs' })).getByRole(
      'textbox'
    ),
    'arn:aws:iam::123456789012:role/admin'
  );
  await user.type(
    within(screen.getByRole('group', { name: 'Azure Identities' })).getByRole(
      'textbox'
    ),
    '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/admin'
  );
  await user.type(
    within(
      screen.getByRole('group', { name: 'GCP Service Accounts' })
    ).getByRole('textbox'),
    'admin@some-project.iam.gserviceaccount.com'
  );
  expect(onChange).toHaveBeenLastCalledWith({
    kind: 'app',
    labels: [{ name: 'env', value: 'prod' }],
    awsRoleARNs: ['arn:aws:iam::123456789012:role/admin'],
    azureIdentities: [
      '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/admin',
    ],
    gcpServiceAccounts: ['admin@some-project.iam.gserviceaccount.com'],
  } as AppAccessSpec);
});

test('DatabaseAccessSpecSection', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(
    <StatefulSection<DatabaseAccessSpec>
      component={DatabaseAccessSpecSection}
      defaultValue={newAccessSpec('db')}
      onChange={onChange}
    />
  );

  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  await user.type(screen.getByPlaceholderText('label key'), 'env');
  await user.type(screen.getByPlaceholderText('label value'), 'prod');
  await selectEvent.create(screen.getByLabelText('Database Names'), 'stuff', {
    createOptionText: 'Database Name: stuff',
  });
  await selectEvent.create(screen.getByLabelText('Database Users'), 'mary', {
    createOptionText: 'Database User: mary',
  });
  await selectEvent.create(screen.getByLabelText('Database Roles'), 'admin', {
    createOptionText: 'Database Role: admin',
  });
  expect(onChange).toHaveBeenLastCalledWith({
    kind: 'db',
    labels: [{ name: 'env', value: 'prod' }],
    names: [expect.objectContaining({ label: 'stuff', value: 'stuff' })],
    roles: [expect.objectContaining({ label: 'admin', value: 'admin' })],
    users: [expect.objectContaining({ label: 'mary', value: 'mary' })],
  } as DatabaseAccessSpec);
});

test('WindowsDesktopAccessSpecSection', async () => {
  const user = userEvent.setup();
  const onChange = jest.fn();
  render(
    <StatefulSection<WindowsDesktopAccessSpec>
      component={WindowsDesktopAccessSpecSection}
      defaultValue={newAccessSpec('windows_desktop')}
      onChange={onChange}
    />
  );

  await user.click(screen.getByRole('button', { name: 'Add a Label' }));
  await user.type(screen.getByPlaceholderText('label key'), 'os');
  await user.type(screen.getByPlaceholderText('label value'), 'win-xp');
  await selectEvent.create(screen.getByLabelText('Logins'), 'julio', {
    createOptionText: 'Login: julio',
  });
  expect(onChange).toHaveBeenLastCalledWith({
    kind: 'windows_desktop',
    labels: [{ name: 'os', value: 'win-xp' }],
    logins: [expect.objectContaining({ label: 'julio', value: 'julio' })],
  } as WindowsDesktopAccessSpec);
});

const reactSelectValueContainer = (input: HTMLInputElement) =>
  // eslint-disable-next-line testing-library/no-node-access
  input.closest('.react-select__value-container');
