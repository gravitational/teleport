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

import React, { DependencyList } from 'react';

import { render } from 'design/utils/testing';

import { useRefAutoFocus } from './useRefAutoFocus';

test('focus automatically when allowed', () => {
  const element = {
    focus: jest.fn(),
  };
  render(<Focusable element={element} shouldFocus={true} />);
  expect(element.focus).toHaveBeenCalledTimes(1);
});

test('do nothing when focus in not allowed', () => {
  const element = {
    focus: jest.fn(),
  };
  render(<Focusable element={element} shouldFocus={false} />);
  expect(element.focus).not.toHaveBeenCalled();
});

test('refocus when deps list changes', () => {
  const element = {
    focus: jest.fn(),
  };
  const { rerender } = render(
    <Focusable
      element={element}
      shouldFocus={true}
      reFocusDeps={['old prop']}
    />
  );
  rerender(
    <Focusable
      element={element}
      shouldFocus={true}
      reFocusDeps={['new prop']}
    />
  );
  expect(element.focus).toHaveBeenCalledTimes(2);
});

test('do not refocus when deps list does not change', () => {
  const element = {
    focus: jest.fn(),
  };
  const { rerender } = render(
    <Focusable
      element={element}
      shouldFocus={true}
      reFocusDeps={['old prop']}
    />
  );
  rerender(
    <Focusable
      element={element}
      shouldFocus={true}
      reFocusDeps={['old prop']}
    />
  );
  expect(element.focus).toHaveBeenCalledTimes(1);
});

const Focusable = (props: {
  element: { focus(): void };
  shouldFocus: boolean;
  reFocusDeps?: DependencyList;
}) => {
  const ref = useRefAutoFocus({
    shouldFocus: props.shouldFocus,
    refocusDeps: props.reFocusDeps,
  });
  ref.current = props.element;
  return null;
};
