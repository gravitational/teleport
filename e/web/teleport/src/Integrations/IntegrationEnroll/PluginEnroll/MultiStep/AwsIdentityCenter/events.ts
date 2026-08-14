import {
  IntegrationEnrollEvent,
  IntegrationEnrollKind,
  IntegrationEnrollStep,
  IntegrationEnrollStepStatus,
  userEventService,
} from 'teleport/services/userEvent';

/**
 * emitEvent emmits integration enroll step events for
 * Identity Center plugin.
 * @param eventId unique ID for the event. For a given integration enroll session.
 *                ID must be reused between steps to ensure correlation.
 * @param step name of the step.
 * @param status is status of the step outcome.
 */
export function emitEvent(
  eventId: string,
  step: IntegrationEnrollStep,
  status: IntegrationEnrollStepStatus
) {
  userEventService.captureIntegrationEnrollEvent({
    event: IntegrationEnrollEvent.Step,
    eventData: {
      id: eventId,
      kind: IntegrationEnrollKind.AwsIdentityCenter,
      step: step,
      status: status,
    },
  });
}
