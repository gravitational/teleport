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

import { useState } from 'react';

import { Box } from 'design';
import { NewFlow, StepSlider } from 'design/StepSlider';

import { OnboardCard } from 'teleport/components/Onboard';
import OSSRecoveryCodes from 'teleport/components/RecoveryCodes';
import cfg from 'teleport/config';
import { loginFlows } from 'teleport/Welcome/NewCredentials/constants';

import useToken from '../useToken';
import { Expired } from './Expired';
import { RegisterSuccess } from './Success';
import { LoginFlow, NewCredentialsProps } from './types';

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
    isDashboard,
    displayOnboardingQuestionnaire = false,
    setDisplayOnboardingQuestionnaire = false,
    Questionnaire = undefined,
    displayInviteCollaborators = false,
    setDisplayInviteCollaborators = null,
    InviteCollaborators = undefined,
    RecoveryCodes,
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

  if (
    success &&
    !resetMode &&
    displayInviteCollaborators &&
    setDisplayInviteCollaborators &&
    InviteCollaborators
  ) {
    return (
      <OnboardCard>
        <InviteCollaborators
          onSubmit={() => setDisplayInviteCollaborators(false)}
        />
      </OnboardCard>
    );
  }

  if (
    success &&
    !resetMode &&
    displayOnboardingQuestionnaire &&
    setDisplayOnboardingQuestionnaire &&
    Questionnaire
  ) {
    return (
      <OnboardCard>
        <Questionnaire
          username={resetToken.user}
          onSubmit={() => setDisplayOnboardingQuestionnaire(false)}
          onboard={true}
        />
      </OnboardCard>
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
    // TODO(bl-nero): Remove OSSRecoveryCodes once the enterprise code passes
    // the RecoveryCodesComponent through props.
    const Component = RecoveryCodes || OSSRecoveryCodes;
    return (
      <Component
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
    <Box as="form">
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
    </Box>
  );
}
