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

import { act } from '@testing-library/react';
import selectEvent from 'react-select-event';

import { render, screen, userEvent } from 'design/utils/testing';
import { Validator } from 'shared/components/Validation';

import { ResourceKind } from 'teleport/services/resources';

import { AdminRules } from './AdminRules';
import { RuleModel } from './standardmodel';
import { StatefulSectionWithDispatch } from './StatefulSection';
import { ActionType, StandardModelDispatcher } from './useStandardModel';
import { AdminRuleValidationResult } from './validation';

describe('AdminRules', () => {
  const setup = () => {
    const modelRef = jest.fn();
    let validator: Validator;
    let dispatch: StandardModelDispatcher;
    render(
      <StatefulSectionWithDispatch<RuleModel[], AdminRuleValidationResult[]>
        selector={m => m.roleModel.rules}
        validationSelector={m => m.validationResult.rules}
        component={AdminRules}
        validatorRef={v => {
          validator = v;
        }}
        modelRef={modelRef}
        dispatchRef={d => {
          dispatch = d;
        }}
      />
    );
    return { user: userEvent.setup(), modelRef, dispatch, validator };
  };

  test('editing', async () => {
    const { user, modelRef } = setup();
    await user.click(screen.getByRole('button', { name: 'Add New' }));
    await selectEvent.select(screen.getByLabelText('Teleport Resources *'), [
      'db',
      'node',
    ]);
    await user.click(screen.getByLabelText('list'));
    await user.click(screen.getByLabelText('read'));
    await user.type(screen.getByLabelText('Filter'), 'some-filter');
    expect(modelRef).toHaveBeenLastCalledWith([
      {
        id: expect.any(String),
        resources: [
          { label: 'db', value: ResourceKind.Database },
          { label: 'node', value: ResourceKind.Node },
        ],
        allVerbs: false,
        verbs: [
          { verb: 'read', checked: true },
          { verb: 'list', checked: true },
          { verb: 'create', checked: false },
          { verb: 'update', checked: false },
          { verb: 'delete', checked: false },
        ],
        where: 'some-filter',
        hideValidationErrors: true,
      },
    ] as RuleModel[]);

    // Add another resource kind and check that the list of permissions got
    // extended.
    expect(screen.queryByLabelText('readnosecrets')).not.toBeInTheDocument();
    await selectEvent.select(screen.getByLabelText('Teleport Resources *'), [
      'db',
      'node',
      'saml',
    ]);
    await user.click(screen.getByLabelText('readnosecrets'));
    expect(modelRef).toHaveBeenLastCalledWith([
      {
        id: expect.any(String),
        resources: [
          { label: 'db', value: ResourceKind.Database },
          { label: 'node', value: ResourceKind.Node },
          { label: 'saml', value: ResourceKind.SAMLConnector },
        ],
        allVerbs: false,
        verbs: [
          { verb: 'read', checked: true },
          { verb: 'list', checked: true },
          { verb: 'create', checked: false },
          { verb: 'update', checked: false },
          { verb: 'delete', checked: false },
          { verb: 'readnosecrets', checked: true },
        ],
        where: 'some-filter',
        hideValidationErrors: true,
      },
    ] as RuleModel[]);

    // Select "All". Expect everything else to be checked.
    await user.click(screen.getByLabelText('All (wildcard verb “*”)'));
    expect(screen.getByLabelText('read')).toBeChecked();
    expect(screen.getByLabelText('list')).toBeChecked();
    expect(screen.getByLabelText('create')).toBeChecked();
    expect(screen.getByLabelText('update')).toBeChecked();
    expect(screen.getByLabelText('delete')).toBeChecked();
    expect(screen.getByLabelText('readnosecrets')).toBeChecked();
    expect(modelRef).toHaveBeenLastCalledWith([
      expect.objectContaining({ allVerbs: true } as Partial<RuleModel>),
    ]);

    // Add one more resource type, expecting one more permission to be
    // available. As now "All" is checked, expect the newly added permission to
    // be checked, too.
    await selectEvent.select(screen.getByLabelText('Teleport Resources *'), [
      'db',
      'node',
      'saml',
      'integration',
    ]);
    expect(screen.getByLabelText('use')).toBeChecked();

    // Uncheck one of the checked verbs. Expect "all" to be automatically
    // unchecked.
    await user.click(screen.getByLabelText('update'));
    expect(screen.getByLabelText('All (wildcard verb “*”)')).not.toBeChecked();
    expect(modelRef).toHaveBeenLastCalledWith([
      expect.objectContaining({
        allVerbs: false,
        verbs: [
          { verb: 'read', checked: true },
          { verb: 'list', checked: true },
          { verb: 'create', checked: true },
          { verb: 'update', checked: false },
          { verb: 'delete', checked: true },
          { verb: 'readnosecrets', checked: true },
          { verb: 'use', checked: true },
        ],
      } as Partial<RuleModel>),
    ]);
  });

  test('validation', async () => {
    const { user, validator, dispatch } = setup();
    await user.click(screen.getByRole('button', { name: 'Add New' }));
    act(() => validator.validate());

    // Validation hidden
    expect(
      screen.queryByText('At least one resource kind is required')
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText('At least one permission is required')
    ).not.toBeInTheDocument();

    // Validation visible
    act(() => dispatch({ type: ActionType.EnableValidation }));
    expect(
      screen.getByText('At least one resource kind is required')
    ).toBeInTheDocument();
    expect(
      screen.getByText('At least one permission is required')
    ).toBeInTheDocument();
  });
});
