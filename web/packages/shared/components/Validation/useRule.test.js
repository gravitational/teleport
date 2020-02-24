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
import Validation, { useRule } from '.';
import { render, fireEvent, wait, screen } from 'design/utils/testing';

test('basic usage', async () => {
  let results;
  await wait(() => {
    results = render(<Component value="" rule={required} />);
  });

  // expect no errors when not validating
  expect(results.container.firstChild.textContent).toBe('valid');

  // trigger validation and expect errors
  fireEvent.click(screen.getByRole('button'));
  expect(results.container.firstChild.textContent).toBe(
    'this field is required'
  );

  // rerender component with proper value and expect no errors
  results.rerender(<Component value="123" rule={required} />);
  expect(results.container.firstChild.textContent).toBe('valid');

  // verify that useRule properly unsubscribes from validation context
  const cb = jest.fn();
  const rule = () => () => cb();
  results.unmount();
  results.rerender(<Component value="123" rule={rule} />);
  results.rerender(<Component visible={false} />);

  fireEvent.click(screen.getByRole('button'));
  expect(cb).not.toHaveBeenCalled();
});

// Component to setup Validation context
function Component(props) {
  const { value, rule, visible = true } = props;
  return (
    <Validation>
      {({ validator }) => (
        <>
          {visible && <ValueBox value={value} rule={rule} />}
          <button role="button" onClick={() => validator.validate()} />
        </>
      )}
    </Validation>
  );
}

// Component to test useRule
function ValueBox({ value, rule }) {
  const results = useRule(rule(value));
  return (
    <>
      {results.valid && 'valid'}
      {!results.valid && results.message}
    </>
  );
}

// sample rule
const required = value => () => {
  return {
    valid: !!value,
    message: !value ? 'this field is required' : '',
  };
};
