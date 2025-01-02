import { useState } from 'react';

import cfg from 'e-teleport/config';
import { RecoveryCodes } from 'e-teleport/RecoveryCodes';
import { InviteCollaboratorsCard } from 'e-teleport/Welcome/InviteCollaborators/InviteCollaboratorsCard';
import { Questionnaire } from 'e-teleport/Welcome/Questionnaire/Questionnaire';
import { useLocation } from 'teleport/components/Router';
import { CLOUD_INVITE_URL_PARAM } from 'teleport/Welcome/const';
import { NewCredentialsContainerProps } from 'teleport/Welcome/NewCredentials';
import { NewCredentials } from 'teleport/Welcome/NewCredentials/NewCredentials';
import useToken from 'teleport/Welcome/useToken';

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
    useState(cfg.oss.hasQuestionnaire && cfg.oss.isCloud);

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
      RecoveryCodes={RecoveryCodes}
    />
  );
}
