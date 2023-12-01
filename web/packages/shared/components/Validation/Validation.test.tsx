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

import React from 'react';

import { render, fireEvent, screen } from 'design/utils/testing';

import Logger from '../../libs/logger';

import Validator, { Validation, useValidation } from './Validation';

jest.mock('../../libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

test('methods of Validator: sub, unsub, validate', () => {
  const mockCb1 = jest.fn();
  const mockCb2 = jest.fn();
  const validator = new Validator();

  // test suscribe
  validator.subscribe(mockCb1);
  validator.subscribe(mockCb2);

  // test validate runs all subscribed cb's
  expect(validator.validate()).toBe(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(1);
  jest.clearAllMocks();

  // test unsubscribe method removes correct cb
  validator.unsubscribe(mockCb2);
  expect(validator.validate()).toBe(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(0);
});

test('methods of Validator: addResult, reset', () => {
  const validator = new Validator();

  // test addResult for nil object
  const result = null;
  validator.addResult(result);
  expect(Logger.create().error).toHaveBeenCalledTimes(1);

  // test addResult for boolean
  validator.addResult(true);
  expect(validator.valid).toBe(false);

  // test addResult with incorrect object
  let resultObj = {};
  validator.addResult(resultObj);
  expect(validator.valid).toBe(false);

  // test addResult with correct object with "valid" prop from prior test set to false
  resultObj = { valid: true };
  validator.addResult(resultObj);
  expect(validator.valid).toBe(false);

  // test reset
  validator.reset();
  expect(validator.valid).toBe(true);
  expect(validator.validating).toBe(false);

  // test addResult with correct object with "valid" prop reset to true
  validator.addResult(resultObj);
  expect(validator.valid).toBe(true);
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
  expect(validator.validating).toBe(true);
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
  expect(validator.validating).toBe(true);
});
