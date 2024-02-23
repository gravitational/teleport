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

import { render, screen } from 'design/utils/testing';

import React from 'react';

import Validation from 'shared/components/Validation';

import { RoleProps, TeamOption } from './types';
import { Role } from './Role';

const makeProps = (): RoleProps => {
  return {
    role: undefined,
    team: undefined,
    teamName: '',
    updateFields: () => {},
  };
};

test('hides custom team input for explicit fields', () => {
  const props = makeProps();
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.queryByLabelText('Team Name')).not.toBeInTheDocument();
});

test('shows custom team input', () => {
  const props = makeProps();
  props.team = 'OTHER' as TeamOption;
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.getByLabelText('Team Name')).toBeInTheDocument();
});
