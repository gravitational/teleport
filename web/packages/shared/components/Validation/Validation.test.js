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
import Validator, { Validation, useValidation } from './Validation';
import { render, fireEvent } from 'design/utils/testing';
import Logger from '../../libs/logger';

jest.mock('../../libs/logger', () => {
  return {
    create: jest.fn(() => ({ error: jest.fn() })),
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
  expect(validator.validate()).toEqual(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(1);
  mockCb1.mockClear();
  mockCb2.mockClear();

  // test unsubscribe method removes correct cb
  validator.unsubscribe(mockCb2);
  expect(validator.validate()).toEqual(true);
  expect(mockCb1).toHaveBeenCalledTimes(1);
  expect(mockCb2).toHaveBeenCalledTimes(0);
});

test('methods of Validator: addResult, reset', () => {
  const validator = new Validator();

  // test addResult for nil object
  const result = null;
  validator.addResult(result);
  expect(Logger.create).toHaveBeenCalledTimes(1);

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

test('render Validation w/ non-func children', () => {
  let mockFn = null;

  // access validator by using context (useValidation)
  const TestButton = prop => {
    const validator = useValidation();

    mockFn = jest.fn(() => validator.validate());

    return <button onClick={mockFn}>{prop.children}</button>;
  };

  const { getByText } = render(
    <Validation>
      <TestButton>hello</TestButton>
    </Validation>
  );

  fireEvent.click(getByText(/hello/i));
  expect(mockFn).toHaveBeenCalledTimes(1);
  expect(mockFn).toHaveReturnedWith(true);
});

test('render Validation with function children', () => {
  const mockFn = jest.fn(validator => validator.validate());

  const { getByText } = render(
    <Validation>
      {({ validator }) => (
        <button onClick={() => mockFn(validator)}>hello</button>
      )}
    </Validation>
  );

  fireEvent.click(getByText(/hello/i));
  expect(mockFn).toHaveBeenCalledTimes(1);
  expect(mockFn).toHaveReturnedWith(true);
});
