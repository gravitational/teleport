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
import FormPassword from './FormPassword';
import { render, fireEvent, wait } from 'design/utils/testing';

jest.mock('../../libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

const placeholdCurrPass = /^password$/i;
const placeholdNewPass = /new password/i;
const placeholdConfirm = /confirm password/i;

const btnSubmitText = /update password/i;

const inputValText = 'aaaaaa';
const inputVal = { target: { value: inputValText } };

test('input validation error states', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithU2f = jest.fn().mockResolvedValue(null);

  const { getByText } = render(
    <FormPassword
      auth2faType={'otp'}
      onChangePass={onChangePass}
      onChangePassWithU2f={onChangePassWithU2f}
    />
  );

  // test input validation error states
  await wait(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).not.toHaveBeenCalled();
  expect(onChangePassWithU2f).not.toHaveBeenCalled();

  expect(getByText(/current password is required/i)).toBeInTheDocument();
  expect(getByText(/enter at least 6 characters/i)).toBeInTheDocument();
  expect(getByText(/please confirm your password/i)).toBeInTheDocument();
  expect(getByText(/token is required/i)).toBeInTheDocument();
});

test('prop auth2faType: disabled', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithU2f = jest.fn().mockResolvedValue(null);

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType="off"
      onChangePass={onChangePass}
      onChangePassWithU2f={onChangePassWithU2f}
    />
  );

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);

  // test the correct cb is called when submitting
  await wait(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).toHaveBeenCalledWith(inputValText, inputValText, '');
  expect(onChangePassWithU2f).not.toHaveBeenCalled();

  // test rendering of status message after submit
  expect(getByText(/your password has been changed!/i)).toBeInTheDocument();

  // test clearing of form values after submit
  expect(getByPlaceholderText(placeholdCurrPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdNewPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdConfirm)).toHaveAttribute('value', '');
});

test('prop auth2faType: OTP form', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithU2f = jest.fn().mockResolvedValue(null);

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType="otp"
      onChangePass={onChangePass}
      onChangePassWithU2f={onChangePassWithU2f}
    />
  );

  // test input validation error state
  await wait(() => fireEvent.click(getByText(btnSubmitText)));

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);
  fireEvent.change(getByPlaceholderText(/otp token/i), inputVal);

  // test the correct cb is called when submitting
  await wait(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).toHaveBeenCalledWith(
    inputValText,
    inputValText,
    inputValText
  );
  expect(onChangePassWithU2f).not.toHaveBeenCalled();

  // test clearing of form values after submit
  expect(getByPlaceholderText(placeholdCurrPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdNewPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdConfirm)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(/otp token/i)).toHaveAttribute('value', '');
});

test('prop auth2faType: U2f form with mocked error', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithU2f = jest.fn().mockRejectedValue(new Error('errMsg'));

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType={'u2f'}
      onChangePass={onChangePass}
      onChangePassWithU2f={onChangePassWithU2f}
    />
  );

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);

  // test U2F status message

  await wait(() => {
    fireEvent.click(getByText(btnSubmitText));
    const statusMsg = getByText(
      /Insert your U2F key and press the button on the key/i
    );
    expect(statusMsg).toBeInTheDocument();
  });

  // test correct cb is called
  expect(onChangePass).not.toHaveBeenCalled();
  expect(onChangePassWithU2f).toHaveBeenCalledTimes(1);

  // test rendering of status message after submit
  expect(getByText(/errMsg/i)).toBeInTheDocument();

  // test forms are NOT cleared with processing errors
  expect(getByPlaceholderText(placeholdCurrPass)).not.toHaveAttribute(
    'value',
    ''
  );
  expect(getByPlaceholderText(placeholdNewPass)).not.toHaveAttribute(
    'value',
    ''
  );
  expect(getByPlaceholderText(placeholdConfirm)).not.toHaveAttribute(
    'value',
    ''
  );
});
