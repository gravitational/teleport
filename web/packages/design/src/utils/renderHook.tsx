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
import { act, render } from '@testing-library/react';

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
