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

import { act, render } from '@testing-library/react';
import React from 'react';

/**
 * @deprecated Use renderHook provided by @testing-library/react-hooks instead.
 */
export default function renderHook<T extends (...args: any) => any>(
  hooks: T,
  options?: Options
) {
  const result = {
    current: null as ReturnType<T>,
  };

  act(() => {
    const Wrapper = options?.wrapper || DefaultWrapper;

    // passes hooksCb to component and expect this component to execute the hook
    // and assign the return values to results.current
    render(
      <Wrapper {...options?.wrapperProps}>
        <TestHook hooksCb={hooks} result={result} />{' '}
      </Wrapper>
    );
  });
  // return hooks results only
  return result;
}

export { act };

// A wrapper to execute hooks during renderer process
function TestHook(props: any) {
  // trigger hooks and assessing its results to props.current
  props.result.current = props.hooksCb();
  return null;
}

function DefaultWrapper(props: { children: any }) {
  return props.children;
}

type Options = {
  wrapper?: React.ComponentType;
  wrapperProps?: any;
};
