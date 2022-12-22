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
