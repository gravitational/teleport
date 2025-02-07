import { useState } from 'react';

import { ConfigureAccess } from './ConfigureAccess';
import { ConfigureSshCert } from './ConfigureSshCert';
import { CreateIntegration } from './CreateIntegration';

const Steps = [CreateIntegration, ConfigureSshCert, ConfigureAccess];

export function GitHub() {
  const [gitHubOrgName, setGitHubOrgName] = useState('');
  const [currStep, setCurrStep] = useState(0);

  function nextStep() {
    setCurrStep(currStep + 1);
  }

  function prevStep() {
    setCurrStep(currStep - 1);
  }

  const CurrComponent = Steps[currStep];

  return (
    <CurrComponent
      gitHubOrgName={gitHubOrgName}
      onGitHubOrgNameChange={setGitHubOrgName}
      nextStep={nextStep}
      prevStep={prevStep}
    />
  );
}

export type Props = {
  gitHubOrgName: string;
  onGitHubOrgNameChange(s: string): void;
  nextStep(): void;
  prevStep(): void;
};
