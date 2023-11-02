import React, { useState } from 'react';

import { useLocation } from 'teleport/components/Router';
import { NewCredentialsContainerProps } from 'teleport/Welcome/NewCredentials';

import useToken from 'teleport/Welcome/useToken';

import { CLOUD_INVITE_URL_PARAM } from 'teleport/Welcome/const';
import { NewCredentials } from 'teleport/Welcome/NewCredentials/NewCredentials';

import cfg from 'e-teleport/config';
import { Questionnaire } from 'e-teleport/Welcome/Questionnaire/Questionnaire';
import { InviteCollaboratorsCard } from 'e-teleport/Welcome/InviteCollaborators/InviteCollaboratorsCard';

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

  const { search } = useLocation();
  const hasInitialUserFlag = new URLSearchParams(search).has(
    CLOUD_INVITE_URL_PARAM
  );

  // Note: we only show this with the "initial" search param set. We can't
  // otherwise determine if this user is actually the first, so we'll have to
  // rely on this indirect option. Users will just be shown an error toast on
  // first login if they lack appropriate invite permissions.
  const [displayInviteCollaborators, setDisplayInviteCollaborators] = useState(
    cfg.oss.isCloud && hasInitialUserFlag
  );

  return (
    <NewCredentials
      {...state}
      resetMode={resetMode}
      isDashboard={cfg.oss.isDashboard}
      displayOnboardingQuestionnaire={displayOnboardingQuestionnaire}
      setDisplayOnboardingQuestionnaire={setDisplayOnboardingQuestionnaire}
      Questionnaire={Questionnaire}
      displayInviteCollaborators={displayInviteCollaborators}
      setDisplayInviteCollaborators={setDisplayInviteCollaborators}
      InviteCollaborators={InviteCollaboratorsCard}
    />
  );
}
