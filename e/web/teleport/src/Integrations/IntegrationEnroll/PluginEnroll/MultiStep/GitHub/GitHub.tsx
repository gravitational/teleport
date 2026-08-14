import { useEffect, useState } from 'react';

import { emitIntegrationStepEvent } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/events';
import cfg from 'teleport/config';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
  IntegrationEnrollStepStatus,
  userEventService,
} from 'teleport/services/userEvent';

import { ConfigureAccess } from './ConfigureAccess';
import { ConfigureSshCert } from './ConfigureSshCert';
import { CreateIntegration } from './CreateIntegration';

const Steps = [
  {
    component: CreateIntegration,
    event: IntegrationEnrollStep.GitHubRaCreateIntegration,
  },
  {
    component: ConfigureSshCert,
    event: IntegrationEnrollStep.GitHubRaConfigureSshCert,
  },
  {
    component: ConfigureAccess,
    event: IntegrationEnrollStep.GitHubRaCreateRole,
  },
];

export function GitHub() {
  const [gitHubOrgName, setGitHubOrgName] = useState('');
  const [currStep, setCurrStep] = useState(0);
  const [eventId] = useState(() => crypto.randomUUID());
  const [enrollComplete, setEnrollComplete] = useState(false);

  useEffect(() => {
    emitEvent({ event: IntegrationEnrollEvent.Started });
  }, []);

  useEffect(() => {
    if (enrollComplete) {
      return;
    }

    const emitAbortEvent = () => {
      if (!enrollComplete) {
        emitEvent({ status: { code: IntegrationEnrollStatusCode.Aborted } });
      }
    };

    // Emit abort event upon refreshing, going to different route
    // (eg: copy and paste url) from same page, or closing tab/browser.
    // Does not capture unmounting edge cases which is handled
    // with the unmount logic below.
    window.addEventListener('beforeunload', emitAbortEvent);

    return () => {
      // Emit abort event upon unmounting from going back or
      // forward to a non-discover route.
      if (
        window.location.pathname !== cfg.getIntegrationEnrollRoute('github')
      ) {
        emitAbortEvent();
      }

      window.removeEventListener('beforeunload', emitAbortEvent);
    };
  }, [enrollComplete]);

  function emitEvent(props: EmitEvent) {
    if ('status' in props) {
      if (
        Steps[currStep].event === IntegrationEnrollStep.GitHubRaCreateRole &&
        props.status.code === IntegrationEnrollStatusCode.Skipped
      ) {
        setEnrollComplete(true);
      }
      emitIntegrationStepEvent({
        eventId,
        step: Steps[currStep].event,
        status: props.status,
        kind: IntegrationEnrollKind.GitHubRepoAccess,
      });
    } else {
      if (props.event === IntegrationEnrollEvent.Complete) {
        setEnrollComplete(true);
      }
      userEventService.captureIntegrationEnrollEvent({
        event: props.event,
        eventData: {
          id: eventId,
          kind: IntegrationEnrollKind.GitHubRepoAccess,
        },
      });
    }
  }

  function nextStep() {
    setCurrStep(currStep + 1);
  }

  function prevStep() {
    setCurrStep(currStep - 1);
  }

  const CurrComponent = Steps[currStep].component;

  return (
    <CurrComponent
      gitHubOrgName={gitHubOrgName}
      onGitHubOrgNameChange={setGitHubOrgName}
      nextStep={nextStep}
      prevStep={prevStep}
      emitEvent={emitEvent}
    />
  );
}

export type EmitEvent =
  | { status: IntegrationEnrollStepStatus }
  | { event: IntegrationEnrollEvent };

export type Props = {
  gitHubOrgName: string;
  onGitHubOrgNameChange(s: string): void;
  nextStep(): void;
  prevStep(): void;
  emitEvent(event: EmitEvent): void;
};
