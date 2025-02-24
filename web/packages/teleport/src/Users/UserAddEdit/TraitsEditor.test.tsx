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

import { emptyTrait, TraitsEditor, type TraitsOption } from './TraitsEditor';
import { traitsToTraitsOption } from './useDialog';

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
