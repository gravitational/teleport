import { WelcomeWrapper, OnboardCard } from 'teleport/components/Onboard';

import { Questionnaire } from './Questionnaire';

export default {
  title: 'TeleportE/Welcome/Questionnaire',
  args: { userContext: true },
};

export const Full = () => {
  return (
    <WelcomeWrapper>
      <OnboardCard>
        <Questionnaire onboard={false} />
      </OnboardCard>
    </WelcomeWrapper>
  );
};
