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
import { act, within } from '@testing-library/react';
import { Validator } from 'shared/components/Validation';
import selectEvent from 'react-select-event';

import {
  AppAccessSpec,
  DatabaseAccessSpec,
  KubernetesAccessSpec,
  newAccessSpec,
  ServerAccessSpec,
  WindowsDesktopAccessSpec,
} from './standardmodel';
import { AccessSpecValidationResult, validateAccessSpec } from './validation';
import {
  ServerAccessSpecSection,
  KubernetesAccessSpecSection,
  AppAccessSpecSection,
  DatabaseAccessSpecSection,
  WindowsDesktopAccessSpecSection,
} from './Resources';
import { StatefulSection } from './StatefulSection';

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
        expect.objectContaining({
          label: '{{internal.logins}}',
          value: '{{internal.logins}}',
        }),
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
        expect.objectContaining({ value: '{{internal.kubernetes_groups}}' }),
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

  const awsRoleArns = () =>
    screen.getByRole('group', { name: 'AWS Role ARNs' });
  const awsRoleArnTextBoxes = () =>
    within(awsRoleArns()).getAllByRole('textbox');
  const azureIdentities = () =>
    screen.getByRole('group', { name: 'Azure Identities' });
  const azureIdentityTextBoxes = () =>
    within(azureIdentities()).getAllByRole('textbox');
  const gcpServiceAccounts = () =>
    screen.getByRole('group', { name: 'GCP Service Accounts' });
  const gcpServiceAccountTextBoxes = () =>
    within(gcpServiceAccounts()).getAllByRole('textbox');

  test('editing', async () => {
    const { user, onChange } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.type(screen.getByPlaceholderText('label key'), 'env');
    await user.type(screen.getByPlaceholderText('label value'), 'prod');
    await user.click(
      within(awsRoleArns()).getByRole('button', { name: 'Add More' })
    );
    await user.type(
      awsRoleArnTextBoxes()[1],
      'arn:aws:iam::123456789012:role/admin'
    );
    await user.click(
      within(azureIdentities()).getByRole('button', { name: 'Add More' })
    );
    await user.type(
      azureIdentityTextBoxes()[1],
      '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/admin'
    );
    await user.click(
      within(gcpServiceAccounts()).getByRole('button', { name: 'Add More' })
    );
    await user.type(
      gcpServiceAccountTextBoxes()[1],
      'admin@some-project.iam.gserviceaccount.com'
    );
    expect(onChange).toHaveBeenLastCalledWith({
      kind: 'app',
      labels: [{ name: 'env', value: 'prod' }],
      awsRoleARNs: [
        '{{internal.aws_role_arns}}',
        'arn:aws:iam::123456789012:role/admin',
      ],
      azureIdentities: [
        '{{internal.azure_identities}}',
        '/subscriptions/1020304050607-cafe-8090-a0b0c0d0e0f0/resourceGroups/example-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/admin',
      ],
      gcpServiceAccounts: [
        '{{internal.gcp_service_accounts}}',
        'admin@some-project.iam.gserviceaccount.com',
      ],
    } as AppAccessSpec);
  });

  test('validation', async () => {
    const { user, validator } = setup();
    await user.click(screen.getByRole('button', { name: 'Add a Label' }));
    await user.click(
      within(awsRoleArns()).getByRole('button', { name: 'Add More' })
    );
    await user.type(awsRoleArnTextBoxes()[1], '*');
    await user.click(
      within(azureIdentities()).getByRole('button', { name: 'Add More' })
    );
    await user.type(azureIdentityTextBoxes()[1], '*');
    await user.click(
      within(gcpServiceAccounts()).getByRole('button', { name: 'Add More' })
    );
    await user.type(gcpServiceAccountTextBoxes()[1], '*');
    act(() => validator.validate());
    expect(
      screen.getByPlaceholderText('label key')
    ).toHaveAccessibleDescription('required');
    expect(awsRoleArnTextBoxes()[1]).toHaveAccessibleDescription(
      'Wildcard is not allowed in AWS role ARNs'
    );
    expect(azureIdentityTextBoxes()[1]).toHaveAccessibleDescription(
      'Wildcard is not allowed in Azure identities'
    );
    expect(gcpServiceAccountTextBoxes()[1]).toHaveAccessibleDescription(
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
      names: [
        expect.objectContaining({ value: '{{internal.db_names}}' }),
        expect.objectContaining({ label: 'stuff', value: 'stuff' }),
      ],
      roles: [
        expect.objectContaining({ value: '{{internal.db_roles}}' }),
        expect.objectContaining({ label: 'admin', value: 'admin' }),
      ],
      users: [
        expect.objectContaining({ value: '{{internal.db_users}}' }),
        expect.objectContaining({ label: 'mary', value: 'mary' }),
      ],
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
      logins: [
        expect.objectContaining({ value: '{{internal.windows_logins}}' }),
        expect.objectContaining({ label: 'julio', value: 'julio' }),
      ],
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

const reactSelectValueContainer = (input: HTMLInputElement) =>
  // eslint-disable-next-line testing-library/no-node-access
  input.closest('.react-select__value-container');
