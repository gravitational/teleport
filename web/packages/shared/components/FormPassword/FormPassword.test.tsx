/**
 * Copyright 2020-2022 Gravitational, Inc.
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
import { On, Optional } from './FormPassword.story';
import { render, fireEvent, waitFor, screen } from 'design/utils/testing';

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
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  const { getByText } = render(
    <FormPassword
      auth2faType={'otp'}
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // test input validation error states
  await waitFor(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).not.toHaveBeenCalled();
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  expect(getByText(/current password is required/i)).toBeInTheDocument();
  expect(getByText(/enter at least 6 characters/i)).toBeInTheDocument();
  expect(getByText(/please confirm your password/i)).toBeInTheDocument();
  expect(getByText(/token is required/i)).toBeInTheDocument();
});

test('prop auth2faType: off', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType="off"
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(screen.queryByTestId('mfa-select')).toBeNull();

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);

  // test the correct cb is called when submitting
  await waitFor(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).toHaveBeenCalledWith(inputValText, inputValText, '');
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  // test rendering of status message after submit
  expect(getByText(/your password has been changed!/i)).toBeInTheDocument();

  // test clearing of form values after submit
  expect(getByPlaceholderText(placeholdCurrPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdNewPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdConfirm)).toHaveAttribute('value', '');
});

test('prop auth2faType: webauthn form with mocked error', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest
    .fn()
    .mockRejectedValue(new Error('errMsg'));

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType={'webauthn'}
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);

  // test correct cb is called
  await waitFor(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePassWithWebauthn).toHaveBeenCalledTimes(1);

  // test rendering of status message after submit
  expect(getByText(/errMsg/i)).toBeInTheDocument();
});

test('prop auth2faType: OTP form', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  const { getByText, getByPlaceholderText } = render(
    <FormPassword
      auth2faType="otp"
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // rendering of mfa dropdown
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // test input validation error state
  await waitFor(() => fireEvent.click(getByText(btnSubmitText)));

  // fill out form
  fireEvent.change(getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(getByPlaceholderText(placeholdConfirm), inputVal);
  fireEvent.change(getByPlaceholderText(/123 456/i), inputVal);

  // test the correct cb is called when submitting
  await waitFor(() => fireEvent.click(getByText(btnSubmitText)));
  expect(onChangePass).toHaveBeenCalledWith(
    inputValText,
    inputValText,
    inputValText
  );
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  // test clearing of form values after submit
  expect(getByPlaceholderText(placeholdCurrPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdNewPass)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(placeholdConfirm)).toHaveAttribute('value', '');
  expect(getByPlaceholderText(/123 456/i)).toHaveAttribute('value', '');
});

test('auth2faType "optional" should render form with hardware key as first option in dropdown', async () => {
  const { container } = render(<Optional />);
  expect(container).toMatchSnapshot();
});

test('auth2faType "on" should render form with hardware key as first option in dropdown', async () => {
  const { container } = render(<On />);
  expect(container).toMatchSnapshot();
});
