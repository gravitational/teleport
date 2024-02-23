import React from 'react';

import { WelcomeWrapper } from 'design/Onboard/WelcomeWrapper';
import { OnboardCard } from 'design/Onboard/OnboardCard';

import { Questionnaire } from './Questionnaire';

export default {
  title: 'Teleport/Welcome/Questionnaire',
  args: { userContext: true },
};

export const Full = () => {
  return (
    <WelcomeWrapper>
      <OnboardCard>
        <Questionnaire onSubmit={() => null} />
      </OnboardCard>
    </WelcomeWrapper>
  );
};
