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

import Dialog from 'design/Dialog';
import { makeEmptyAttempt } from 'shared/hooks/useAsync';

import { ContextProvider } from 'teleport';
import { ReauthState } from 'teleport/components/ReAuthenticate/useReAuthenticate';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  MFA_OPTION_SSO_DEFAULT,
  MFA_OPTION_TOTP,
  MFA_OPTION_WEBAUTHN,
  WebauthnAssertionResponse,
} from 'teleport/services/mfa';

import {
  ChangePasswordStep,
  ChangePasswordWizardStepProps,
  ReauthenticateStep,
} from './ChangePasswordWizard';

export default {
  title: 'teleport/Account/Manage Devices/Change Password Wizard',
  decorators: [
    Story => {
      const ctx = createTeleportContext();
      return (
        <ContextProvider ctx={ctx}>
          <Dialog
            open={true}
            dialogCss={() => ({ width: '650px', padding: 0 })}
          >
            <Story />
          </Dialog>
        </ContextProvider>
      );
    },
  ],
};

export function Reauthenticate() {
  return <ReauthenticateStep {...stepProps} />;
}

export function ChangePasswordWithPasswordlessVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="passwordless" />;
}

export function ChangePasswordWithMfaDeviceVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="webauthn" />;
}

export function ChangePasswordWithOtpVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="totp" />;
}

export function ChangePasswordWithSsoVerification() {
  return <ChangePasswordStep {...stepProps} reauthMethod="sso" />;
}

const stepProps = {
  // StepComponentProps
  next() {},
  prev() {},
  hasTransitionEnded: true,
  stepIndex: 0,
  flowLength: 2,
  refCallback: () => {},

  // Shared props
  reauthMethod: 'passwordless',
  onClose() {},
  onSuccess() {},

  // ReauthenticateStepProps
  hasPasswordless: true,
  setReauthMethod: () => {},
  reauthState: {
    initAttempt: { status: 'success' },
    mfaOptions: [MFA_OPTION_WEBAUTHN, MFA_OPTION_TOTP, MFA_OPTION_SSO_DEFAULT],
    submitWithMfa: async () => null,
    submitAttempt: makeEmptyAttempt(),
    clearSubmitAttempt: () => {},
  } as ReauthState,

  // ChangePasswordStepProps
  webauthnResponse: {} as WebauthnAssertionResponse,
} satisfies ChangePasswordWizardStepProps;
