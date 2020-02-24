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
import { create, act } from 'react-test-renderer';

// A wrapper to execute hooks during renderer process
function TestHook(props) {
  // trigger hooks and assessing its results to props.current
  props.result.current = props.hooksCb();
  return null;
}

export default function renderHook(hooksCb) {
  const result = {
    current: null,
  };
  act(() => {
    // passes hooksCb to component and expect this
    // component to pass back hooks execution results via results.results
    create(<TestHook hooksCb={hooksCb} result={result} />);
  });
  // return hooks results only
  return result;
}

export { act };
