/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { render, screen } from 'design/utils/testing';

import React from 'react';

import Validation from 'shared/components/Validation';

import { RoleProps } from './types';
import { Role } from './Role';

const makeProps = (): RoleProps => {
  return {
    role: 'C-Suite/Owner',
    team: 'DevOps Engineering',
    teamName: '',
    updateFields: () => {},
  };
};

test('hides custom team input for explicit fields', () => {
  const props = makeProps();
  props.team = 'DevOps Engineering';
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.queryByLabelText('Team Name')).not.toBeInTheDocument();
});

test('shows custom team input', () => {
  const props = makeProps();
  props.team = 'Other (free-form field)';
  render(
    <Validation>
      <Role {...props} />
    </Validation>
  );

  expect(screen.getByLabelText('Team Name')).toBeInTheDocument();
});
