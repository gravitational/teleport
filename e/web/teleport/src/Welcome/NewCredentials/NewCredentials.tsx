import React, { useState } from 'react';

import { NewCredentialsContainerProps } from 'teleport/Welcome/NewCredentials';

import useToken from 'teleport/Welcome/useToken';

import { NewCredentials } from 'teleport/Welcome/NewCredentials/NewCredentials';

import cfg from 'e-teleport/config';
import { Questionnaire } from 'e-teleport/Welcome/Questionnaire/Questionnaire';

/**
 *
 * @remarks
 * This container component is duplicated in OSS for OSS onboarding. If you are making edits to this file, check to see if the
 * equivalent change should be applied in OSS
 *
 */

export function Container({
  tokenId = '',
  resetMode = false,
}: NewCredentialsContainerProps) {
  const state = useToken(tokenId);
  const [displayOnboardingQuestionnaire, setDisplayOnboardingQuestionnaire] =
    useState(cfg.oss.isUsageBasedBilling && cfg.oss.isCloud);

  return (
    <NewCredentials
      {...state}
      resetMode={resetMode}
      isDashboard={cfg.oss.isDashboard}
      displayOnboardingQuestionnaire={displayOnboardingQuestionnaire}
      setDisplayOnboardingQuestionnaire={setDisplayOnboardingQuestionnaire}
      Questionnaire={Questionnaire}
    />
  );
}
