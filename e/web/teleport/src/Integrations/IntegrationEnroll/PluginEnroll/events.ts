import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  IntegrationEnrollStep,
  IntegrationEnrollStepStatus,
  userEventService,
} from 'teleport/services/userEvent';

export function emitIntegrationStepEvent({
  eventId,
  step,
  status,
  kind,
}: {
  /**
   * unique ID for the event.
   * ID must be reused between steps to ensure correlation.
   */
  eventId: string;
  /**
   * name of the step that we are emitting an event for.
   */
  step: IntegrationEnrollStep;
  /**
   * status of the step outcome.
   */
  status: IntegrationEnrollStepStatus;
  /**
   * the integration kind that is being enrolled.
   */
  kind: IntegrationEnrollKind;
}) {
  userEventService.captureIntegrationEnrollEvent({
    event: IntegrationEnrollEvent.Step,
    eventData: {
      id: eventId,
      kind: kind,
      step: step,
      status: status,
    },
  });
}
