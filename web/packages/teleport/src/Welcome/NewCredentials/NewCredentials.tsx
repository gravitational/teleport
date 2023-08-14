/*
Copyright 2021-2022 Gravitational, Inc.

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

import React, { useState } from 'react';
import { Card } from 'design';
import { PrimaryAuthType } from 'shared/services';

import { NewFlow, StepComponentProps, StepSlider } from 'design/StepSlider';

import RecoveryCodes from 'teleport/components/RecoveryCodes';
import { PrivateKeyLoginDisabledCard } from 'teleport/components/PrivateKeyPolicy';

import useToken, { State } from '../useToken';

import { Expired } from './Expired';
import { RegisterSuccess } from './Success';
import { NewMfaDevice } from './NewMfaDevice';
import { NewPasswordlessDevice } from './NewPasswordlessDevice';
import { NewPassword } from './NewPassword';

export type LoginFlow = Extract<PrimaryAuthType, 'passwordless' | 'local'>;
export type SliderProps = StepComponentProps & {
  changeFlow(f: NewFlow<LoginFlow>): void;
};

const loginFlows = {
  local: [NewPassword, NewMfaDevice],
  passwordless: [NewPasswordlessDevice],
};

export function Container({ tokenId = '', resetMode = false }) {
  const state = useToken(tokenId);
  return <NewCredentials {...state} resetMode={resetMode} />;
}

export function NewCredentials(props: State & Props) {
  const {
    fetchAttempt,
    recoveryCodes,
    resetMode,
    resetToken,
    redirect,
    primaryAuthType,
    success,
    finishedRegister,
    privateKeyPolicyEnabled,
  } = props;

  if (fetchAttempt.status === 'failed') {
    return <Expired resetMode={resetMode} />;
  }

  if (fetchAttempt.status !== 'success') {
    return null;
  }

  if (success && privateKeyPolicyEnabled) {
    return (
      <PrivateKeyLoginDisabledCard
        title={resetMode ? 'Reset Complete' : 'Registration Complete'}
      />
    );
  }

  if (success) {
    return (
      <RegisterSuccess
        redirect={redirect}
        resetMode={resetMode}
        username={resetToken.user}
      />
    );
  }

  if (recoveryCodes) {
    return (
      <RecoveryCodes
        recoveryCodes={recoveryCodes}
        onContinue={finishedRegister}
        isNewCodes={resetMode}
        username={resetToken.user}
      />
    );
  }

  // Check which flow to render as default.
  const [password, setPassword] = useState('');
  const [newFlow, setNewFlow] = useState<NewFlow<LoginFlow>>();
  const [flow, setFlow] = useState<LoginFlow>(() => {
    if (primaryAuthType === 'sso' || primaryAuthType === 'local') {
      return 'local';
    }
    return 'passwordless';
  });

  function onSwitchFlow(flow: LoginFlow) {
    setFlow(flow);
  }

  function onNewFlow(flow: NewFlow<LoginFlow>) {
    setNewFlow(flow);
  }

  function updatePassword(password: string) {
    setPassword(password);
  }

  return (
    <Card as="form" my={5} mx="auto" width={464}>
      <StepSlider<typeof loginFlows>
        flows={loginFlows}
        currFlow={flow}
        onSwitchFlow={onSwitchFlow}
        newFlow={newFlow}
        changeFlow={onNewFlow}
        {...props}
        password={password}
        updatePassword={updatePassword}
      />
    </Card>
  );
}

export type Props = State & {
  resetMode?: boolean;
};
