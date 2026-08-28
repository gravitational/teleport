/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { render, screen, userEvent } from 'design/utils/testing';

import { NewMfaDeviceForm, NewMfaDeviceFormProps } from './NewMfaDeviceForm';

function formProps(
  overrides: Partial<NewMfaDeviceFormProps> = {}
): NewMfaDeviceFormProps {
  return {
    title: 'Form Title',
    submitButtonText: 'Submit Button',
    submitAttempt: { status: '' },
    clearSubmitAttempt: () => {},
    qrCode: '',
    auth2faType: 'optional',
    createNewWebAuthnDevice: () => {},
    onSubmitWithWebAuthn: () => {},
    onSubmit: () => {},
    shouldFocus: false,
    stepIndex: 1,
    flowLength: 3,
    ...overrides,
  };
}

test('renders title', () => {
  render(<NewMfaDeviceForm {...formProps()} />);
  expect(screen.getByText('Step 2 of 3')).toBeVisible();
  expect(screen.getByText('Form Title')).toBeVisible();
});

test('back arrow', async () => {
  const user = userEvent.setup();
  render(<NewMfaDeviceForm {...formProps()} />);
  expect(
    screen.queryByRole('button', { name: /back/i })
  ).not.toBeInTheDocument();

  const prev = jest.fn();
  const clearSubmitAttempt = jest.fn();
  render(<NewMfaDeviceForm {...formProps({ prev, clearSubmitAttempt })} />);
  await user.click(screen.getByRole('button', { name: /back/i }));
  expect(prev).toHaveBeenCalled();
  expect(clearSubmitAttempt).toHaveBeenCalled();
});

test('MFA options', () => {
  render(<NewMfaDeviceForm {...formProps({ auth2faType: 'otp' })} />);
  expect(
    screen.queryByLabelText(/passkey or security key/i)
  ).not.toBeInTheDocument();
});

test('WebAuthn', async () => {
  const user = userEvent.setup();
  const createNewWebAuthnDevice = jest.fn();
  const onSubmitWithWebAuthn = jest.fn();
  let props = formProps({ createNewWebAuthnDevice, onSubmitWithWebAuthn });
  const { rerender } = render(<NewMfaDeviceForm {...props} />);

  await user.click(screen.getByLabelText(/passkey or security key/i));
  await user.click(screen.getByText(/create an MFA method/i));
  expect(createNewWebAuthnDevice).toHaveBeenCalled();

  props = { ...props, credential: { id: '', type: 'public-key' } };
  rerender(<NewMfaDeviceForm {...props} />);

  const methodNameInput = screen.getByLabelText(/MFA method name/i);
  expect(methodNameInput).toHaveValue('webauthn-device');
  await user.clear(methodNameInput);
  await user.type(methodNameInput, 'new-device');
  await user.click(screen.getByText('Submit Button'));
  expect(onSubmitWithWebAuthn).toHaveBeenCalledWith('new-device');
});

test('OTP', async () => {
  const user = userEvent.setup();
  const onSubmit = jest.fn();
  render(<NewMfaDeviceForm {...formProps({ onSubmit, qrCode: 'qr-code' })} />);

  await user.click(screen.getByLabelText(/authenticator app/i));
  const methodNameInput = screen.getByLabelText(/MFA method name/i);
  expect(methodNameInput).toHaveValue('otp-device');
  expect(screen.getByRole('img')).toHaveAttribute(
    'src',
    'data:image/png;base64,qr-code'
  );

  await user.clear(methodNameInput);
  await user.type(methodNameInput, 'new-app');
  await user.type(screen.getByLabelText(/authenticator code/i), '546823');
  await user.click(screen.getByText('Submit Button'));
  expect(onSubmit).toHaveBeenCalledWith('546823', 'new-app');
});

test('no MFA', async () => {
  const user = userEvent.setup();
  render(<NewMfaDeviceForm {...formProps()} />);

  await user.click(screen.getByLabelText(/none/i));
  expect(screen.getByText(/we strongly recommend enrolling/i)).toBeVisible();
});

test('error message', async () => {
  render(
    <NewMfaDeviceForm
      {...formProps({
        submitAttempt: { status: 'failed', statusText: 'Oh, no! Anyway...' },
      })}
    />
  );

  expect(screen.getByText('Oh, no! Anyway...')).toBeVisible();
});
