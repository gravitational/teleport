import React, { useContext, useState } from 'react';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import useTeleportE from 'e-teleport/useTeleportE';
import {
  ExternalAuditStorage,
  Integration,
  integrationService,
} from 'teleport/services/integrations';

export enum Step {
  SelectIntegration = 0,
  ConfigurePermissions,
  TestConnection,
  Activation,
}

type ExternalAuditStorageContext = {
  currentStep: Step;
  setCurrentStep: (step: Step) => void;
  nextStep: () => void;
  selectedAwsIntegration: Integration;
  setSelectedAwsIntegration: (i: Integration) => void;
  draft: ExternalAuditStorage;
  createDraft: () => Promise<void>;
  continuePreviousDraft: () => Promise<boolean>;
  attempt: Attempt;
};
const stepsContext = React.createContext<ExternalAuditStorageContext>(null);

type StepsProps = unknown;
export function ExternalAuditStorageProvider({
  children,
}: React.PropsWithChildren<StepsProps>) {
  const { externalAuditStorageService } = useTeleportE();
  const [currentStep, setCurrentStep] = useState<Step>(Step.SelectIntegration);
  const [draft, setDraft] = useState<ExternalAuditStorage>(null);
  const { attempt, run } = useAttempt();

  const [selectedAwsIntegration, setSelectedAwsIntegration] =
    useState<Integration>(null);

  function nextStep() {
    setCurrentStep(currentStep + 1);
  }

  function continuePreviousDraft() {
    return run(() =>
      externalAuditStorageService.getDraft().then(result => {
        run(() => {
          return integrationService
            .fetchIntegration(result.integrationName)
            .then(integration => {
              setSelectedAwsIntegration(integration);
              setDraft(result);
            });
        });
      })
    );
  }

  function createDraft() {
    return externalAuditStorageService
      .generateDraft(selectedAwsIntegration.name)
      .then(generated => {
        setDraft(generated);
      });
  }

  const value = {
    currentStep,
    setCurrentStep,
    nextStep,
    selectedAwsIntegration,
    setSelectedAwsIntegration,
    draft,
    createDraft,
    continuePreviousDraft,
    attempt,
  };

  return (
    <stepsContext.Provider value={value}>{children}</stepsContext.Provider>
  );
}

export function useExternalAuditStorage(): ExternalAuditStorageContext {
  return useContext(stepsContext);
}
