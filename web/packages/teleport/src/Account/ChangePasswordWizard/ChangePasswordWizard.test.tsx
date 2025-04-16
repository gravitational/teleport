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

import { waitFor, within } from '@testing-library/react';
import { userEvent, UserEvent } from '@testing-library/user-event';

import { render, screen } from 'design/utils/testing';

import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';
import {
  MFA_OPTION_TOTP,
  MFA_OPTION_WEBAUTHN,
  MfaChallengeResponse,
} from 'teleport/services/mfa';

import { ChangePasswordWizard } from '.';
import {
  ChangePasswordWizardProps,
  getReauthOptions,
  REAUTH_OPTION_PASSKEY,
  REAUTH_OPTION_WEBAUTHN,
} from './ChangePasswordWizard';

const dummyChallengeResponse: MfaChallengeResponse = {
  webauthn_response: {
    id: 'cred-id',
    type: 'public-key',
    extensions: {
      appid: true,
    },
    rawId: 'rawId',
    response: {
      authenticatorData: 'authenticatorData',
      clientDataJSON: 'clientDataJSON',
      signature: 'signature',
      userHandle: 'userHandle',
    },
  },
};
let user: UserEvent;
let onSuccess: jest.Mock;

function TestWizard(props: Partial<ChangePasswordWizardProps> = {}) {
  return (
    <ChangePasswordWizard
      hasPasswordless={true}
      onClose={() => {}}
      onSuccess={onSuccess}
      {...props}
    />
  );
}

beforeEach(() => {
  user = userEvent.setup();
  onSuccess = jest.fn();

  jest.spyOn(auth, 'getMfaChallenge').mockResolvedValue({
    totpChallenge: true,
    webauthnPublicKey: {} as PublicKeyCredentialRequestOptions,
  });
  jest
    .spyOn(auth, 'getMfaChallengeResponse')
    .mockResolvedValueOnce(dummyChallengeResponse);
  jest.spyOn(auth, 'changePassword').mockResolvedValueOnce(undefined);
});

afterEach(jest.resetAllMocks);

describe('with passwordless reauthentication', () => {
  async function reauthenticate() {
    render(<TestWizard />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });
    expect(auth.getMfaChallenge).toHaveBeenCalledWith({
      scope: MfaChallengeScope.CHANGE_PASSWORD,
    });

    await user.click(screen.getByText('Passkey'));
    await user.click(screen.getByText('Next'));
    expect(auth.getMfaChallenge).toHaveBeenCalledWith({
      scope: MfaChallengeScope.CHANGE_PASSWORD,
      userVerificationRequirement: 'required',
    });
    expect(auth.getMfaChallengeResponse).toHaveBeenCalled();
  }

  it('changes password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).toHaveBeenCalledWith({
      oldPassword: '',
      newPassword: 'new-pass1234',
      mfaResponse: {
        totp_code: '',
        webauthn_response: dummyChallengeResponse.webauthn_response,
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  it('cancels changing password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Back'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('validates the password form', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass123'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass123'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(changePasswordStep.getByLabelText('New Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('New Password')
    ).toHaveAccessibleDescription('Enter at least 12 characters');

    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(changePasswordStep.getByLabelText('Confirm Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('Confirm Password')
    ).toHaveAccessibleDescription('Password does not match');
  });
});

describe('with WebAuthn MFA reauthentication', () => {
  async function reauthenticate() {
    render(<TestWizard />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });
    expect(auth.getMfaChallenge).toHaveBeenCalledWith({
      scope: MfaChallengeScope.CHANGE_PASSWORD,
    });

    await user.click(screen.getByText('Security Key'));
    await user.click(screen.getByText('Next'));
    expect(auth.getMfaChallengeResponse).toHaveBeenCalled();
  }

  it('changes password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('Current Password'),
      'current-pass'
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).toHaveBeenCalledWith({
      oldPassword: 'current-pass',
      newPassword: 'new-pass1234',
      mfaResponse: {
        totp_code: '',
        webauthn_response: dummyChallengeResponse.webauthn_response,
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  it('cancels changing password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Back'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('validates the password form', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass123'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass123'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(changePasswordStep.getByLabelText('New Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('New Password')
    ).toHaveAccessibleDescription('Enter at least 12 characters');
    expect(changePasswordStep.getByLabelText('Current Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('Current Password')
    ).toHaveAccessibleDescription('Current Password is required');

    await user.type(
      changePasswordStep.getByLabelText('Current Password'),
      'current-pass'
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(
      changePasswordStep.getByLabelText('Confirm Password')
    ).toHaveAccessibleDescription('Password does not match');
  });
});

describe('with OTP MFA reauthentication', () => {
  async function reauthenticate() {
    render(<TestWizard />);

    await waitFor(() => {
      expect(screen.getByTestId('reauthenticate-step')).toBeInTheDocument();
    });
    await user.click(screen.getByText('Authenticator App'));
    await user.click(screen.getByText('Next'));
  }

  it('changes password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('Current Password'),
      'current-pass'
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText(/Authenticator Code/),
      '654321'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).toHaveBeenCalledWith({
      oldPassword: 'current-pass',
      newPassword: 'new-pass1234',
      mfaResponse: {
        totp_code: '654321',
      },
    });
    expect(onSuccess).toHaveBeenCalled();
  });

  it('cancels changing password', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Back'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('validates the password form', async () => {
    await reauthenticate();
    const changePasswordStep = within(
      screen.getByTestId('change-password-step')
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass123'
    );
    await user.type(
      changePasswordStep.getByLabelText('Confirm Password'),
      'new-pass123'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(changePasswordStep.getByLabelText('New Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('New Password')
    ).toHaveAccessibleDescription('Enter at least 12 characters');
    expect(changePasswordStep.getByLabelText('Current Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('Current Password')
    ).toHaveAccessibleDescription('Current Password is required');
    expect(
      changePasswordStep.getByLabelText('Authenticator Code')
    ).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('Authenticator Code')
    ).toHaveAccessibleDescription('Authenticator code is required');

    await user.type(
      changePasswordStep.getByLabelText('Authenticator Code'),
      '654321'
    );
    await user.type(
      changePasswordStep.getByLabelText('Current Password'),
      'current-pass'
    );
    await user.type(
      changePasswordStep.getByLabelText('New Password'),
      'new-pass1234'
    );
    await user.click(changePasswordStep.getByText('Save Changes'));
    expect(auth.changePassword).not.toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
    expect(changePasswordStep.getByLabelText('Confirm Password')).toBeInvalid();
    expect(
      changePasswordStep.getByLabelText('Confirm Password')
    ).toHaveAccessibleDescription('Password does not match');
  });
});

test.each`
  mfaOptions                                | hasPasswordless | reauthOptions
  ${[MFA_OPTION_TOTP]}                      | ${false}        | ${[MFA_OPTION_TOTP]}
  ${[MFA_OPTION_WEBAUTHN]}                  | ${false}        | ${[REAUTH_OPTION_WEBAUTHN]}
  ${[MFA_OPTION_TOTP, MFA_OPTION_WEBAUTHN]} | ${false}        | ${[MFA_OPTION_TOTP, REAUTH_OPTION_WEBAUTHN]}
  ${[MFA_OPTION_WEBAUTHN]}                  | ${true}         | ${[REAUTH_OPTION_PASSKEY, REAUTH_OPTION_WEBAUTHN]}
  ${[MFA_OPTION_TOTP, MFA_OPTION_WEBAUTHN]} | ${true}         | ${[REAUTH_OPTION_PASSKEY, MFA_OPTION_TOTP, REAUTH_OPTION_WEBAUTHN]}
`(
  'getReauthOptions: mfaOptions=$mfaOptions, hasPasswordless=$hasPasswordless, devices=$deviceCase',
  ({ mfaOptions, hasPasswordless, reauthOptions }) => {
    const gotReauthOptions = getReauthOptions(mfaOptions, hasPasswordless);
    expect(gotReauthOptions).toEqual(reauthOptions);
  }
);
