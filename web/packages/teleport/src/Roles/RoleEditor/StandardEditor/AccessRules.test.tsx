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
import { act } from '@testing-library/react';
import { Validator } from 'shared/components/Validation';
import selectEvent from 'react-select-event';
import { ResourceKind } from 'teleport/services/resources';

import { RuleModel } from './standardmodel';
import { AccessRuleValidationResult, validateAccessRule } from './validation';
import { AccessRules } from './AccessRules';
import { StatefulSection } from './StatefulSection';

describe('AccessRules', () => {
  const setup = () => {
    const onChange = jest.fn();
    let validator: Validator;
    render(
      <StatefulSection<RuleModel[], AccessRuleValidationResult[]>
        component={AccessRules}
        defaultValue={[]}
        onChange={onChange}
        validatorRef={v => {
          validator = v;
        }}
        validate={rules => rules.map(validateAccessRule)}
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
