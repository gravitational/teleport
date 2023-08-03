import React from 'react';

import { Questionnaire } from './Questionnaire';

export default {
  title: 'TeleportE/Welcome/Questionnaire',
  args: { userContext: true },
};

export const Full = () => {
  return <Questionnaire onboard={false} />;
};
