import React from 'react';

import { Questionnaire } from './Questionnaire';

export default {
  title: 'TeleportE/Welcome/Questionnaire',
  args: { userContext: true },
};

export const Full = () => {
  return <Questionnaire full={true} username={''} />;
};

export const Partial = () => {
  return <Questionnaire full={false} username={''} />;
};
