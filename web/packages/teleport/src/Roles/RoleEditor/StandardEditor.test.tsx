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

import { act, within } from '@testing-library/react';
import Validation, { Validator } from 'shared/components/Validation';
import selectEvent from 'react-select-event';

import TeleportContextProvider from 'teleport/TeleportContextProvider';
import { createTeleportContext } from 'teleport/mocks/contexts';

import { ResourceKind } from 'teleport/services/resources';

import {
  AppAccessSpec,
  DatabaseAccessSpec,
  KubernetesAccessSpec,
  newAccessSpec,
  newRole,
  roleToRoleEditorModel,
  RuleModel,
  ServerAccessSpec,
  StandardEditorModel,
  WindowsDesktopAccessSpec,
} from './standardmodel';
import {
  AdminRules,
  AppAccessSpecSection,
  DatabaseAccessSpecSection,
  KubernetesAccessSpecSection,
  SectionProps,
  ServerAccessSpecSection,
  StandardEditor,
  StandardEditorProps,
  WindowsDesktopAccessSpecSection,
} from './StandardEditor';
import {
  AccessSpecValidationResult,
  AdminRuleValidationResult,
  validateAccessSpec,
  validateAdminRule,
} from './validation';

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
  await user.click(screen.getByRole('tab', { name: 'Resources' }));
  expect(getAllSectionNames()).toEqual([]);

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
  expect(getAllSectionNames()).toEqual(['Servers']);

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

test('invisible tabs still apply validation', async () => {
  const user = userEvent.setup();
  const onSave = jest.fn();
  render(<TestStandardEditor onSave={onSave} />);
  // Intentionally cause a validation error.
  await user.clear(screen.getByLabelText('Role Name'));
  // Switch to a different tab.
  await user.click(screen.getByRole('tab', { name: 'Resources' }));
  await user.click(screen.getByRole('button', { name: 'Create Role' }));
  expect(onSave).not.toHaveBeenCalled();

  // Switch back, make it valid.
  await user.click(screen.getByRole('tab', { name: 'Invalid data Overview' }));
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

function StatefulSection<S, V>({
  defaultValue,
  component: Component,
  onChange,
  validatorRef,
  validate,
}: {
  defaultValue: S;
  component: React.ComponentType<SectionProps<S, any>>;
  onChange(spec: S): void;
  validatorRef?(v: Validator): void;
  validate(arg: S): V;
}) {
  const [model, setModel] = useState<S>(defaultValue);
  const validation = validate(model);
  return (
    <Validation>
      {({ validator }) => {
        validatorRef?.(validator);
        return (
          <Component
            value={model}
            validation={validation}
            isProcessing={false}
            onChange={spec => {
              setModel(spec);
              onChange(spec);
            }}
          />
        );
      }}
    </Validation>
  );
}

describe('ServerAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<ServerAccessSpec, AccessSpecValidationResult>
        component={ServerAccessSpecSection}
        defaultValue={newAccessSpec('node')}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={validateAccessSpec}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
  };

  test('editing', async () => {
    const { user, onChange } = setup();
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

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await selectEvent.create(screen.getByLabelText('Logins'), '*', {
      createOptionText: 'Login: *',
    });
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
    expect(
      screen.getByText('Wildcard is not allowed in logins')
    ).toBeInTheDocument();
  });
});

describe('KubernetesAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<KubernetesAccessSpec, AccessSpecValidationResult>
        component={KubernetesAccessSpecSection}
        defaultValue={newAccessSpec('kube_cluster')}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={validateAccessSpec}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
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

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.click(screen.getByRole('button', { name: 'Add a Resource' }));
    await user.clear(screen.getByLabelText('Name'));
    await user.clear(screen.getByLabelText('Namespace'));
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
    expect(screen.getByLabelText('Name')).toHaveAccessibleDescription(
      'Resource name is required, use "*" for any resource'
    );
    expect(screen.getByLabelText('Namespace')).toHaveAccessibleDescription(
      'Namespace is required for resources of this kind'
    );
  });
});

describe('AppAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<AppAccessSpec, AccessSpecValidationResult>
        component={AppAccessSpecSection}
        defaultValue={newAccessSpec('app')}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={validateAccessSpec}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
  };

  const awsRoleArn = () =>
    within(screen.getByRole('group', { name: 'AWS Role ARNs' })).getByRole(
      'textbox'
    );
  const azureIdentity = () =>
    within(screen.getByRole('group', { name: 'Azure Identities' })).getByRole(
      'textbox'
    );
  const gcpServiceAccount = () =>
    within(
      screen.getByRole('group', { name: 'GCP Service Accounts' })
    ).getByRole('textbox');

  test('editing', async () => {
    const { user, onChange } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.type(screen.getByPlaceholderText('label key'), 'env');
    await user.type(screen.getByPlaceholderText('label value'), 'prod');
    await user.type(awsRoleArn(), 'arn:aws:iam::123456789012:role/admin');
    await user.type(
      azureIdentity(),
      '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/admin'
    );
    await user.type(
      gcpServiceAccount(),
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

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.type(awsRoleArn(), '*');
    await user.type(azureIdentity(), '*');
    await user.type(gcpServiceAccount(), '*');
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
    expect(awsRoleArn()).toHaveAccessibleDescription(
      'Wildcard is not allowed in AWS role ARNs'
    );
    expect(azureIdentity()).toHaveAccessibleDescription(
      'Wildcard is not allowed in Azure identities'
    );
    expect(gcpServiceAccount()).toHaveAccessibleDescription(
      'Wildcard is not allowed in GCP service accounts'
    );
  });
});

describe('DatabaseAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<DatabaseAccessSpec, AccessSpecValidationResult>
        component={DatabaseAccessSpecSection}
        defaultValue={newAccessSpec('db')}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={validateAccessSpec}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
  };

  test('editing', async () => {
    const { user, onChange } = setup();
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

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await selectEvent.create(screen.getByLabelText('Database Roles'), '*', {
      createOptionText: 'Database Role: *',
    });
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
    expect(
      screen.getByText('Wildcard is not allowed in database roles')
    ).toBeInTheDocument();
  });
});

describe('WindowsDesktopAccessSpecSection', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<WindowsDesktopAccessSpec, AccessSpecValidationResult>
        component={WindowsDesktopAccessSpecSection}
        defaultValue={newAccessSpec('windows_desktop')}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={validateAccessSpec}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
  };

  test('editing', async () => {
    const { user, onChange } = setup();
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

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
  });
});

describe('AdminRules', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<RuleModel[], AdminRuleValidationResult[]>
        component={AdminRules}
        defaultValue={[]}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={rules => rules.map(validateAdminRule)}
      />
    );
    return { user: userEvent.setup(), onChange, validator };
  };

  test('editing', async () => {
    const { user, onChange } = setup();
    await user.click(screen.getByRole('button', { name: 'Add New' }));
    await selectEvent.select(screen.getByLabelText('Resources'), [
      'db',
      'node',
    ]);
    await selectEvent.select(screen.getByLabelText('Permissions'), [
      'list',
      'read',
    ]);
    expect(onChange).toHaveBeenLastCalledWith([
      {
        id: expect.any(String),
        resources: [
          { label: ResourceKind.Database, value: 'db' },
          { label: ResourceKind.Node, value: 'node' },
        ],
        verbs: [
          { label: 'list', value: 'list' },
          { label: 'read', value: 'read' },
        ],
      },
    ] as RuleModel[]);
  });

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add New' }));
    act(() => validator.validate());
    expect(
      screen.getByText('At least one resource kind is required')
    ).toBeInTheDocument();
    expect(
      screen.getByText('At least one permission is required')
    ).toBeInTheDocument();
  });
});

const reactSelectValueContainer = (input: HTMLInputElement) =>
  // eslint-disable-next-line testing-library/no-node-access
  input.closest('.react-select__value-container');
