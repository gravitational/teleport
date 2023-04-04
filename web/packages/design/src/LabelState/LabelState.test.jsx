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

import LabelState, {
  StateDanger,
  StateInfo,
  StateWarning,
  StateSuccess,
} from './LabelState';

const colors = {
  primary: theme.colors.brand.main,
  info: theme.colors.levels.sunkenSecondary,
  warning: theme.colors.warning,
  danger: theme.colors.danger,
  success: theme.colors.success,
};

describe('design/LabelState', () => {
  test.each`
    Component       | kind           | expected
    ${LabelState}   | ${'primary'}   | ${colors.primary}
    ${StateInfo}    | ${'secondary'} | ${colors.info}
    ${StateWarning} | ${'warning'}   | ${colors.warning}
    ${StateDanger}  | ${'danger'}    | ${colors.danger}
    ${StateSuccess} | ${'success'}   | ${colors.success}
  `('respects kind prop set to $kind', ({ Component, expected }) => {
    const { container } = render(<Component />);
    expect(container.firstChild).toHaveStyle({
      background: expected,
    });

    expect(getComputedStyle(container.firstChild).boxShadow).toBe('');
  });

  it('respects shadow prop', () => {
    const { container } = render(<LabelState shadow={true} />);
    expect(getComputedStyle(container.firstChild).boxShadow).toEqual(
      expect.any(String)
    );
  });
});
