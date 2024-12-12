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

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import {
  MFA_OPTION_SSO_DEFAULT,
  MFA_OPTION_TOTP,
  WebauthnAssertionResponse,
} from 'teleport/services/mfa';

import {
  ChangePasswordStep,
  ChangePasswordWizardStepProps,
  REAUTH_OPTION_PASSKEY,
  REAUTH_OPTION_WEBAUTHN,
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
  reauthOptions: [
    REAUTH_OPTION_PASSKEY,
    REAUTH_OPTION_WEBAUTHN,
    MFA_OPTION_TOTP,
    MFA_OPTION_SSO_DEFAULT,
  ],
  onReauthMethodChange: () => {},
  submitWithMfa: async () => {},

  // ChangePasswordStepProps
  webauthnResponse: {} as WebauthnAssertionResponse,
} satisfies ChangePasswordWizardStepProps;
