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

import { render, fireEvent, screen } from 'design/utils/testing';

import Validation, { useRule } from '.';

test('basic usage', async () => {
  const utils = render(<Component value="" rule={required} />);

  // expect no errors when not validating
  expect(utils.container.firstChild).toHaveTextContent('valid');

  // trigger validation and expect errors
  fireEvent.click(screen.getByRole('button'));
  expect(utils.container.firstChild).toHaveTextContent(
    'this field is required'
  );

  // rerender component with proper value and expect no errors
  utils.rerender(<Component value="123" rule={required} />);
  expect(utils.container.firstChild).toHaveTextContent('valid');

  // verify that useRule properly unsubscribes from validation context
  const cb = jest.fn();
  const rule = () => () => cb();
  utils.unmount();
  utils.rerender(<Component value="123" rule={rule} />);
  utils.rerender(<Component visible={false} />);

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
  const utils = useRule(rule(value));
  return (
    <>
      {utils.valid && 'valid'}
      {!utils.valid && utils.message}
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
