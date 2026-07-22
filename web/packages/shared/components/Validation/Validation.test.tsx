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

import { useState } from 'react';

import { act, fireEvent, render, screen } from 'design/utils/testing';

import FieldInput from '../FieldInput';
import { requiredField } from './rules';
import Validator, {
  Result,
  useValidation,
  Validation,
  ValidationSuspender,
} from './Validation';

afterEach(() => {
  jest.restoreAllMocks();
});

test('methods of Validator: addRuleCallback, removeRuleCallback, validate', () => {
  const mockCb1 = jest.fn();
  const mockCb2 = jest.fn();
  const validator = new Validator();

  // test suscribe
  validator.addRuleCallback(mockCb1);
  validator.addRuleCallback(mockCb2);

  // test validate runs all subscribed cb's
  expect(validator.validate()).toBe(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(1);
  jest.clearAllMocks();

  // test unsubscribe method removes correct cb
  validator.removeRuleCallback(mockCb2);
  expect(validator.validate()).toBe(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(0);
});

test('methods of Validator: addResult, reset', () => {
  const consoleError = jest.spyOn(console, 'error').mockImplementation();
  const validator = new Validator();

  // test addResult for nil object
  const result = null;
  validator.addResult(result);
  expect(consoleError).toHaveBeenCalledTimes(1);

  // test addResult for boolean
  validator.addResult(true);
  expect(validator.state.valid).toBe(false);

  // test addResult with incorrect object
  validator.addResult({} as Result);
  expect(validator.state.valid).toBe(false);

  // test addResult with correct object with "valid" prop from prior test set to false
  let resultObj = { valid: true };
  validator.addResult(resultObj);
  expect(validator.state.valid).toBe(false);

  // test reset
  validator.reset();
  expect(validator.state.valid).toBe(true);
  expect(validator.state.validating).toBe(false);

  // test addResult with correct object with "valid" prop reset to true
  validator.addResult(resultObj);
  expect(validator.state.valid).toBe(true);
});

test('trigger validation via useValidation hook', () => {
  let validator = null;
  const Button = () => {
    validator = useValidation();
    return <button role="button" onClick={() => validator.validate()} />;
  };

  render(
    <Validation>
      <Button />
    </Validation>
  );
  fireEvent.click(screen.getByRole('button'));
  expect(validator.state.validating).toBe(true);
});

test('trigger validation via render function', () => {
  let validator = null;

  render(
    <Validation>
      {props => (
        <button
          role="button"
          onClick={() => {
            validator = props.validator;
            validator.validate();
          }}
        />
      )}
    </Validation>
  );
  fireEvent.click(screen.getByRole('button'));
  expect(validator.state.validating).toBe(true);
});

test('rendering validation result via useValidation hook', () => {
  let validator: Validator;
  const TestComponent = () => {
    validator = useValidation();
    return (
      <>
        <div>Validating: {String(validator.state.validating)}</div>
        <div>Valid: {String(validator.state.valid)}</div>
      </>
    );
  };
  render(
    <Validation>
      <TestComponent />
    </Validation>
  );
  validator.addRuleCallback(() => validator.addResult({ valid: false }));

  expect(screen.getByText('Validating: false')).toBeInTheDocument();
  expect(screen.getByText('Valid: true')).toBeInTheDocument();

  act(() => validator.validate());
  expect(screen.getByText('Validating: true')).toBeInTheDocument();
  expect(screen.getByText('Valid: false')).toBeInTheDocument();
});

test('rendering validation result via render function', () => {
  let validator: Validator;
  render(
    <Validation>
      {props => {
        validator = props.validator;
        return (
          <>
            <div>Validating: {String(validator.state.validating)}</div>
            <div>Valid: {String(validator.state.valid)}</div>
          </>
        );
      }}
    </Validation>
  );
  validator.addRuleCallback(() => validator.addResult({ valid: false }));

  expect(screen.getByText('Validating: false')).toBeInTheDocument();
  expect(screen.getByText('Valid: true')).toBeInTheDocument();

  act(() => validator.validate());
  expect(screen.getByText('Validating: true')).toBeInTheDocument();
  expect(screen.getByText('Valid: false')).toBeInTheDocument();
});

test('suspending validation', () => {
  const TestComponent = () => {
    const [str1, setStr1] = useState('');
    const [str2, setStr2] = useState('');
    const [suspend, setSuspend] = useState(true);
    return (
      <Validation>
        {({ validator }) => {
          return (
            <>
              <FieldInput
                label="Field 1"
                rule={requiredField('required 1')}
                value={str1}
                onChange={e => setStr1(e.target.value)}
              />
              <ValidationSuspender suspend={suspend}>
                <FieldInput
                  label="Field 2"
                  rule={requiredField('required 2')}
                  value={str2}
                  onChange={e => setStr2(e.target.value)}
                />
              </ValidationSuspender>
              <button onClick={() => validator.validate()}>Validate</button>
              <button onClick={() => setSuspend(s => !s)}>
                Toggle suspend
              </button>
            </>
          );
        }}
      </Validation>
    );
  };

  render(<TestComponent />);
  // Validation of field 2 suspended, only 1st field is validated.
  fireEvent.click(screen.getByRole('button', { name: 'Validate' }));
  expect(screen.queryByText('required 1')).toBeVisible();
  expect(screen.queryByText('required 2')).not.toBeInTheDocument();

  fireEvent.change(screen.getByLabelText('Field 1'), {
    target: { value: 'foo' },
  });
  expect(screen.queryByText('required 1')).not.toBeInTheDocument();
  expect(screen.queryByText('required 2')).not.toBeInTheDocument();

  // Validation of field 2 resumed, both fields are validated.
  fireEvent.click(screen.getByRole('button', { name: 'Toggle suspend' }));
  expect(screen.queryByText('required 1')).not.toBeInTheDocument();
  expect(screen.getByText('required 2')).toBeVisible();

  fireEvent.change(screen.getByLabelText('Field 2'), {
    target: { value: 'foo' },
  });
  expect(screen.queryByText('required 2')).not.toBeInTheDocument();
});
