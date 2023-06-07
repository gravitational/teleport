/*
Copyright 2020-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Card } from 'design';

import { Props, NewCredentials, SliderProps } from './NewCredentials';
import { NewMfaDevice } from './NewMfaDevice';

export default {
  title: 'Teleport/Welcome/Form',
  component: NewCredentials,
};

export const PasswordOnly = () => (
  <NewCredentials {...props} isPasswordlessEnabled={false} />
);

export const PasswordOnlyError = () => (
  <NewCredentials
    {...props}
    isPasswordlessEnabled={false}
    submitAttempt={{
      status: 'failed',
      statusText: 'some server error message',
    }}
  />
);

export const PrimaryPasswordNoMfa = () => <NewCredentials {...props} />;

export const PrimaryPasswordWithMfa = () => (
  <NewCredentials {...props} auth2faType="optional" />
);

export const PrimaryPasswordlessNoMfa = () => (
  <NewCredentials {...props} primaryAuthType="passwordless" />
);

export const PrimaryPasswordlessWithMfa = () => (
  <NewCredentials
    {...props}
    primaryAuthType="passwordless"
    auth2faType="optional"
  />
);

export const PrimaryPasswordlessError = () => (
  <NewCredentials
    {...props}
    primaryAuthType="passwordless"
    submitAttempt={{
      status: 'failed',
      statusText: 'some server error message',
    }}
  />
);

export const MfaDeviceOtp = () => (
  <CardWrapper>
    <NewMfaDevice {...props} {...sliderProps} auth2faType={'otp'} />
  </CardWrapper>
);

export const MfaDeviceWebauthn = () => (
  <CardWrapper>
    <NewMfaDevice {...props} {...sliderProps} auth2faType={'webauthn'} />
  </CardWrapper>
);

export const MfaDeviceOptional = () => (
  <CardWrapper>
    <NewMfaDevice {...props} {...sliderProps} auth2faType={'optional'} />
  </CardWrapper>
);

export const MfaDeviceOn = () => (
  <CardWrapper>
    <NewMfaDevice {...props} {...sliderProps} auth2faType={'on'} />
  </CardWrapper>
);

export const MfaDeviceError = () => (
  <CardWrapper>
    <NewMfaDevice
      {...props}
      {...sliderProps}
      auth2faType={'otp'}
      submitAttempt={{
        status: 'failed',
        statusText: 'some server error message',
      }}
    />
  </CardWrapper>
);

export const ExpiredInvite = () => (
  <NewCredentials {...props} fetchAttempt={{ status: 'failed' }} />
);

export const ExpiredReset = () => (
  <NewCredentials
    {...props}
    fetchAttempt={{ status: 'failed' }}
    resetMode={true}
  />
);
export const RecoveryCodesInvite = () => (
  <NewCredentials {...props} recoveryCodes={recoveryCodes} />
);

export const RecoveryCodesReset = () => (
  <NewCredentials {...props} recoveryCodes={recoveryCodes} resetMode={true} />
);

const recoveryCodes = {
  codes: [
    'tele-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword-testword',
    'tele-testword-testword-testword-testword-testword-testword-testword',
  ],
  createdDate: new Date('2019-08-30T11:00:00.00Z'),
};

export const SuccessRegister = () => (
  <NewCredentials {...props} success={true} />
);
export const SuccessReset = () => (
  <NewCredentials {...props} success={true} resetMode={true} />
);
export const SuccessAndPrivateKeyEnabledRegister = () => (
  <NewCredentials {...props} success={true} privateKeyPolicyEnabled={true} />
);
export const SuccessAndPrivateKeyEnabledReset = () => (
  <NewCredentials
    {...props}
    success={true}
    resetMode={true}
    privateKeyPolicyEnabled={true}
  />
);

function CardWrapper({ children }) {
  return (
    <Card as="form" bg="levels.surface" my={5} mx="auto" width={464}>
      {children}
    </Card>
  );
}

const sliderProps: SliderProps & {
  password: string;
  updatePassword(): void;
} = {
  next: () => null,
  prev: () => null,
  changeFlow: () => null,
  refCallback: () => null,
  hasTransitionEnded: true,
  password: '',
  updatePassword: () => null,
};
const props: Props = {
  auth2faType: 'off',
  primaryAuthType: 'local',
  isPasswordlessEnabled: true,
  submitAttempt: { status: '' },
  clearSubmitAttempt: () => null,
  fetchAttempt: { status: 'success' },
  onSubmitWithWebauthn: () => null,
  onSubmit: () => null,
  redirect: () => null,
  success: false,
  finishedRegister: () => null,
  recoveryCodes: null,
  privateKeyPolicyEnabled: false,
  resetToken: {
    user: 'john@example.com',
    tokenId: 'test123',
    qrCode:
      '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',
  },
};
