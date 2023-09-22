/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { render, theme } from 'design/utils/testing';

import Label, { Primary, Secondary, Warning, Danger } from './Label';

describe('design/Label', () => {
  const colors = [
    theme.colors.brand,
    theme.colors.levels.sunkenSecondary,
    theme.colors.warning.main,
    theme.colors.danger,
  ];

  test.each`
    kind                  | Component    | expected
    ${'default: primary'} | ${Label}     | ${colors[0]}
    ${'primary'}          | ${Primary}   | ${colors[0]}
    ${'secondary'}        | ${Secondary} | ${colors[1]}
    ${'warning'}          | ${Warning}   | ${colors[2]}
    ${'danger'}           | ${Danger}    | ${colors[3]}
  `('component renders $kind label', ({ Component, expected }) => {
    const { container } = render(<Component />);
    expect(container.firstChild).toHaveStyle({
      'background-color': expected,
    });
  });
});
