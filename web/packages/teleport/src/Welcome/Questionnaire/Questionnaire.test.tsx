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

import React from 'react';
import { render, screen } from 'design/utils/testing';

import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import { Questionnaire } from './Questionnaire';
import { QuestionnaireProps } from './types';

describe('questionnaire', () => {
  let props: QuestionnaireProps;

  beforeEach(() => {
    mockUserContextProviderWith(makeTestUserContext());
    props = {
      full: false,
      username: '',
    };
  });

  test('loads each question', () => {
    props.full = true;
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeVisible();
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

  test('skips questions if not full', () => {
    props.full = false;
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();

    expect(screen.queryByLabelText('Company Name')).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText('Number of Employees')
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Which infrastructure resources do you need to access frequently?'
      )
    ).toBeInTheDocument();
  });
});
