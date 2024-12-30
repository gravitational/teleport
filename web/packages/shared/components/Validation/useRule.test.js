/*
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

import { fireEvent, render, screen } from 'design/utils/testing';

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
});

test('valid component with no errors', () => {
  const utils = render(<Component value="123" rule={required} />);

  expect(utils.container.firstChild).toHaveTextContent('valid');
});

test('verify that useRule properly unsubscribes from validation context', () => {
  const cb = jest.fn();
  const rule = () => () => cb();

  const utils = render(<Component value="123" rule={rule} />);
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
