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

import { render, fireEvent, waitFor, screen } from 'design/utils/testing';

import FormPassword from './FormPassword';

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

const inputValText = 'aaaaaaaaaaaa';
const inputVal = { target: { value: inputValText } };

test('input validation error states', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  render(
    <FormPassword
      auth2faType={'otp'}
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // test input validation error states
  fireEvent.click(screen.getByText(btnSubmitText));
  expect(onChangePass).not.toHaveBeenCalled();
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  expect(screen.getByText(/current password is required/i)).toBeInTheDocument();
  expect(screen.getByText(/enter at least 12 characters/i)).toBeInTheDocument();
  expect(screen.getByText(/please confirm your password/i)).toBeInTheDocument();
  expect(screen.getByText(/token is required/i)).toBeInTheDocument();
});

test('prop auth2faType: off', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  render(
    <FormPassword
      auth2faType="off"
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(screen.queryByTestId('mfa-select')).not.toBeInTheDocument();

  // fill out form
  fireEvent.change(screen.getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdConfirm), inputVal);

  // test the correct cb is called when submitting
  fireEvent.click(screen.getByText(btnSubmitText));
  expect(onChangePass).toHaveBeenCalledWith(inputValText, inputValText, '');
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  // test rendering of status message after submit
  await screen.findByText(/your password has been changed!/i);

  // test clearing of form values after submit
  expect(screen.getByPlaceholderText(placeholdCurrPass)).toHaveValue('');
  expect(screen.getByPlaceholderText(placeholdNewPass)).toHaveValue('');
  expect(screen.getByPlaceholderText(placeholdConfirm)).toHaveValue('');
});

test('prop auth2faType: webauthn form with mocked error', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest
    .fn()
    .mockRejectedValue(new Error('errMsg'));

  render(
    <FormPassword
      auth2faType={'webauthn'}
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // Rendering of mfa dropdown.
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // fill out form
  fireEvent.change(screen.getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdConfirm), inputVal);

  // test correct cb is called
  fireEvent.click(screen.getByText(btnSubmitText));
  expect(onChangePassWithWebauthn).toHaveBeenCalledTimes(1);

  // test rendering of status message after submit
  await waitFor(() => {
    expect(screen.getByText(/errMsg/i)).toBeInTheDocument();
  });
});

test('prop auth2faType: OTP form', async () => {
  const onChangePass = jest.fn().mockResolvedValue(null);
  const onChangePassWithWebauthn = jest.fn().mockResolvedValue(null);

  render(
    <FormPassword
      auth2faType="otp"
      onChangePass={onChangePass}
      onChangePassWithWebauthn={onChangePassWithWebauthn}
    />
  );

  // rendering of mfa dropdown
  expect(screen.getByTestId('mfa-select')).not.toBeEmptyDOMElement();

  // test input validation error state
  fireEvent.click(screen.getByText(btnSubmitText));

  // fill out form
  fireEvent.change(screen.getByPlaceholderText(placeholdCurrPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdNewPass), inputVal);
  fireEvent.change(screen.getByPlaceholderText(placeholdConfirm), inputVal);
  fireEvent.change(screen.getByPlaceholderText(/123 456/i), inputVal);

  // test the correct cb is called when submitting
  fireEvent.click(screen.getByText(btnSubmitText));
  expect(onChangePass).toHaveBeenCalledWith(
    inputValText,
    inputValText,
    inputValText
  );
  expect(onChangePassWithWebauthn).not.toHaveBeenCalled();

  // test clearing of form values after submit
  await waitFor(() => {
    expect(screen.getByPlaceholderText(placeholdCurrPass)).toHaveValue('');
  });
  expect(screen.getByPlaceholderText(placeholdNewPass)).toHaveValue('');
  expect(screen.getByPlaceholderText(placeholdConfirm)).toHaveValue('');
  expect(screen.getByPlaceholderText(/123 456/i)).toHaveValue('');
});
