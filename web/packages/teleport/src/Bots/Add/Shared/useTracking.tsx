/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import {
  createContext,
  PropsWithChildren,
  useContext,
  useMemo,
  useRef,
} from 'react';

import { debounce } from 'shared/utils/highbar';

import {
  IntegrationEnrollCodeType,
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollField,
  IntegrationEnrollKind,
  IntegrationEnrollSection,
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent/types';
import { userEventService } from 'teleport/services/userEvent/userEvent';

export function useTracking() {
  const ctx = useContext(context);
  if (!ctx) {
    throw new Error(
      'Tracking not implemented, do you need to add a TrackingProvider?'
    );
  }
  return ctx;
}

export function TrackingProvider(
  props: { initialEventId?: string; disabled?: boolean } & PropsWithChildren
) {
  const { initialEventId = crypto.randomUUID(), disabled = false } = props;

  // Changes to the eventId should not cause a re-render.
  const eventId = useRef(initialEventId);

  const debouncedField = useRef(
    debounce(
      (
        eventId: string,
        step: IntegrationEnrollStep,
        field: IntegrationEnrollField
      ) => {
        return sendEvent(IntegrationEnrollEvent.FieldComplete, {
          id: eventId,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          field,
        });
      },
      1000
    )
  );

  // Keep a stable value as this is likely to be used in effects
  const value = useMemo(
    () => ({
      reset(newId?: string) {
        eventId.current = newId || crypto.randomUUID();
      },
      start() {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.Started, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
        });
      },
      complete() {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.Complete, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
        });
      },
      step(
        step: IntegrationEnrollStep,
        status: Exclude<
          IntegrationEnrollStatusCode,
          IntegrationEnrollStatusCode.Error
        >
      ) {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.Step, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          status: {
            code: status,
          },
        });
      },
      error(step: IntegrationEnrollStep, error: string) {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.Step, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          status: {
            code: IntegrationEnrollStatusCode.Error,
            error,
          },
        });
      },
      field(
        step: IntegrationEnrollStep,
        field: IntegrationEnrollField,
        isEmpty?: boolean
      ) {
        if (disabled || isEmpty) return;
        return debouncedField.current(eventId.current, step, field);
      },
      section(step: IntegrationEnrollStep, section: IntegrationEnrollSection) {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.SectionOpen, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          section,
        });
      },
      link(step: IntegrationEnrollStep, link: string) {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.LinkClick, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          link,
        });
      },
      codeCopy(
        step: IntegrationEnrollStep,
        codeType: IntegrationEnrollCodeType
      ) {
        if (disabled) return;
        return sendEvent(IntegrationEnrollEvent.CodeCopy, {
          id: eventId.current,
          kind: IntegrationEnrollKind.MachineIDGitHubActionsKubernetes,
          step,
          codeType,
        });
      },
    }),
    [disabled]
  );

  return <context.Provider value={value}>{props.children}</context.Provider>;
}

function sendEvent(
  event: IntegrationEnrollEvent,
  eventData: IntegrationEnrollEventData
) {
  userEventService.captureIntegrationEnrollEvent({
    event,
    eventData,
  });
}

type Context = {
  start: () => void;
  complete: () => void;
  step: (
    step: IntegrationEnrollStep,
    statusCode: IntegrationEnrollStatusCode
  ) => void;
  error: (step: IntegrationEnrollStep, error: string) => void;
  /**
   * Track when a field's value changes. This call is debounced across all
   * fields.
   * @param step The step in the guide where the field appears
   * @param field The field's identifier
   * @param isEmpty If the field has no input
   */
  field: (
    step: IntegrationEnrollStep,
    field: IntegrationEnrollField,
    isEmpty?: boolean
  ) => void;
  section: (
    step: IntegrationEnrollStep,
    section: IntegrationEnrollSection
  ) => void;
  link: (step: IntegrationEnrollStep, link: string) => void;
  codeCopy: (
    step: IntegrationEnrollStep,
    codeType: IntegrationEnrollCodeType
  ) => void;
};

const context = createContext<Context | null>(null);
