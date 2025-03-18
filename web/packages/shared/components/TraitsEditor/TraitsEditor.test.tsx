/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { fireEvent, render, screen } from 'design/utils/testing';
import Validation from 'shared/components/Validation';

import { AllUserTraits } from 'teleport/services/user';

import { traitsToTraitsOption, emptyTrait, TraitsEditor, type TraitsOption } from './TraitsEditor';

test('Available traits are rendered', async () => {
  const setConfiguredTraits = jest.fn();
  const userTraits: AllUserTraits = {
    logins: ['root', 'ubuntu'],
    db_roles: ['dbadmin', 'postgres'],
    db_names: ['postgres', 'aurora'],
  };

  render(
    <Validation>
      <TraitsEditor
        attempt={{ status: '' }}
        configuredTraits={traitsToTraitsOption(userTraits)}
        setConfiguredTraits={setConfiguredTraits}
      />
    </Validation>
  );

  expect(screen.getByText('User Traits')).toBeInTheDocument();
  expect(screen.getAllByTestId('trait-key')).toHaveLength(3);
  expect(screen.getAllByTestId('trait-value')).toHaveLength(3);
});

test('Add and remove Trait', async () => {
  const configuredTraits: TraitsOption[] = [];
  const setConfiguredTraits = jest.fn();

  const { rerender } = render(
    <Validation>
      <TraitsEditor
        attempt={{ status: '' }}
        configuredTraits={configuredTraits}
        setConfiguredTraits={setConfiguredTraits}
      />
    </Validation>
  );
  expect(screen.queryAllByTestId('trait-key')).toHaveLength(0);

  const addButtonEl = screen.getByRole('button', { name: /Add user trait/i });
  expect(addButtonEl).toBeInTheDocument();
  fireEvent.click(addButtonEl);

  expect(setConfiguredTraits).toHaveBeenLastCalledWith([emptyTrait]);

  const singleTrait = { logins: ['root', 'ubuntu'] };
  rerender(
    <Validation>
      <TraitsEditor
        attempt={{ status: '' }}
        configuredTraits={traitsToTraitsOption(singleTrait)}
        setConfiguredTraits={setConfiguredTraits}
      />
    </Validation>
  );
  fireEvent.click(screen.getByTitle('Remove Trait'));
  expect(setConfiguredTraits).toHaveBeenCalledWith([]);
});

describe('Test traitsToTraitsOption', () => {
  test.each`
    name                                         | trait                | expected
    ${'trait with values (valid)'}               | ${{ t: ['a', 'b'] }} | ${[{ traitKey: { label: 't', value: 't' }, traitValues: [{ label: 'a', value: 'a' }, { label: 'b', value: 'b' }] }]}
    ${'empty trait (invalid)'}                   | ${{ t: [] }}         | ${[]}
    ${'trait with empty string (invalid)'}       | ${{ t: [''] }}       | ${[]}
    ${'trait with null value (invalid)'}         | ${{ t: null }}       | ${[]}
    ${'trait with null array (invalid)'}         | ${{ t: [null] }}     | ${[]}
    ${'trait with first empty string (invalid)'} | ${{ t: ['', 'a'] }}  | ${[{ traitKey: { label: 't', value: 't' }, traitValues: [{ label: '', value: '' }, { label: 'a', value: 'a' }] }]}
    ${'trait with last empty element (valid)'}   | ${{ t: ['a', ''] }}  | ${[{ traitKey: { label: 't', value: 't' }, traitValues: [{ label: 'a', value: 'a' }, { label: '', value: '' }] }]}
  `('$name', ({ trait, expected }) => {
    const result = traitsToTraitsOption(trait);
    expect(result).toEqual(expected);
  });
});
