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
import { NewFlow, StepSlider } from 'design/StepSlider';

import RecoveryCodes from 'teleport/components/RecoveryCodes';
import { PrivateKeyLoginDisabledCard } from 'teleport/components/PrivateKeyPolicy';
import cfg from 'teleport/config';

import { loginFlows } from 'teleport/Welcome/NewCredentials/constants';

import useToken from '../useToken';

import { Expired } from './Expired';
import { LoginFlow, NewCredentialsProps } from './types';
import { RegisterSuccess } from './Success';

/**
 *
 * @remarks
 * This container component is duplicated in Enterprise for Enterprise onboarding. If you are making edits to this file, check to see if the
 * equivalent change should be applied in Enterprise
 *
 */
export function Container({ tokenId = '', resetMode = false }) {
  const state = useToken(tokenId);
  return (
    <NewCredentials
      {...state}
      resetMode={resetMode}
      isDashboard={cfg.isDashboard}
    />
  );
}

export function NewCredentials(props: NewCredentialsProps) {
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
    isDashboard,
    displayOnboardingQuestionnaire = false,
    setDisplayOnboardingQuestionnaire = false,
    Questionnaire = undefined,
  } = props;

  // Check which flow to render as default.
  const [password, setPassword] = useState('');
  const [newFlow, setNewFlow] = useState<NewFlow<LoginFlow>>();
  const [flow, setFlow] = useState<LoginFlow>(() => {
    if (primaryAuthType === 'sso' || primaryAuthType === 'local') {
      return 'local';
    }
    return 'passwordless';
  });

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

  if (
    success &&
    !resetMode &&
    displayOnboardingQuestionnaire &&
    setDisplayOnboardingQuestionnaire &&
    Questionnaire
  ) {
    // todo (michellescripts) check cluster config to determine if all or partial questions are asked
    return (
      <Questionnaire
        full={true}
        username={resetToken.user}
        onSubmit={() => setDisplayOnboardingQuestionnaire(false)}
      />
    );
  }

  if (success) {
    return (
      <RegisterSuccess
        redirect={redirect}
        resetMode={resetMode}
        username={resetToken.user}
        isDashboard={isDashboard}
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
