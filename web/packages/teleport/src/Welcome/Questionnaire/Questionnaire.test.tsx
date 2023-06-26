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

import { screen } from '@testing-library/react';
import React from 'react';
import { render } from 'design/utils/testing';

import { Questionnaire } from './Questionnaire';

describe('questionnaire', () => {
  test('loads each question', () => {
    render(<Questionnaire username="" />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();

    expect(screen.getByLabelText('Company Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Number of Employees')).toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Which infrastructure resources do you need to access frequently?'
      )
    ).toBeInTheDocument();
  });
});
