/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useContext, useMemo, useState, useEffect } from 'react';

import { useLocation, useHistory } from 'react-router';

import { ResourceKind } from 'teleport/Discover/Shared';
import {
  DiscoverEventStatus,
  DiscoverEventStepStatus,
  DiscoverEvent,
  DiscoverEventResource,
  userEventService,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import { addIndexToViews, findViewAtIndex, Resource, View } from './flow';
import { resourceKindToEventResource } from './Shared/ResourceKind';
import { resources } from './resources';

import type { Node } from 'teleport/services/nodes';
import type { Kube } from 'teleport/services/kube';
import type { Database } from 'teleport/services/databases';
import type { AgentLabel } from 'teleport/services/agents';

export function getKindFromString(value: string) {
  switch (value) {
    case 'application':
      return ResourceKind.Application;
    case 'database':
      return ResourceKind.Database;
    case 'desktop':
      return ResourceKind.Desktop;
    case 'kubernetes':
      return ResourceKind.Kubernetes;
    default:
    case 'server':
      return ResourceKind.Server;
  }
}

interface DiscoverContextState<T = any> {
  agentMeta: AgentMeta;
  currentStep: number;
  nextStep: (count?: number) => void;
  prevStep: () => void;
  onSelectResource: (kind: ResourceKind) => void;
  resourceState: T;
  selectedResource: Resource<T>;
  selectedResourceKind: ResourceKind;
  setResourceState: (value: T) => void;
  updateAgentMeta: (meta: AgentMeta) => void;
  views: View[];
  emitErrorEvent(errorStr: string): void;
  emitEvent(status: DiscoverEventStepStatus): void;
  updateEventState(): void;
  eventState: EventState;
}

type EventState = {
  id: string;
  currEventName: DiscoverEvent;
  resource: DiscoverEventResource;
};

const discoverContext = React.createContext<DiscoverContextState>(null);

export function DiscoverProvider<T = any>(
  props: React.PropsWithChildren<unknown>
) {
  const location = useLocation<{ entity: string }>();
  const history = useHistory();

  const [currentStep, setCurrentStep] = useState(0);
  const [selectedResourceKind, setSelectedResourceKind] =
    useState<ResourceKind>(getKindFromString(location?.state?.entity));
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();
  const [resourceState, setResourceState] = useState<T>();

  const [eventState, setEventState] = useState<EventState>();
  // Use the latest state for async use.
  const ref = React.useRef<EventState>();
  useEffect(() => {
    ref.current = eventState;
  }, [eventState]);

  const selectedResource = resources.find(r => r.kind === selectedResourceKind);

  const views = useMemo<View[]>(() => {
    if (typeof selectedResource.views === 'function') {
      return addIndexToViews(selectedResource.views(resourceState));
    }

    return addIndexToViews(selectedResource.views);
  }, [selectedResource.views, resourceState]);

  useEffect(() => {
    const emitAbortOrSuccessEvent = () => {
      if (ref.current.currEventName === DiscoverEvent.Completed) {
        emitEvent({ stepStatus: DiscoverEventStatus.Success });
      } else {
        emitEvent({ stepStatus: DiscoverEventStatus.Aborted });
      }
    };

    // Emit abort event upon refreshing, going to different route
    // (eg: copy and paste url) from same page, or closing tab/browser.
    // Does not capture unmounting edge cases which is handled
    // with the unmount logic below.
    window.addEventListener('beforeunload', emitAbortOrSuccessEvent);

    return () => {
      // Emit abort event upon unmounting from going back or
      // forward to a non-discover route or upon exiting from
      // the exit prompt.
      if (history.location.pathname !== cfg.routes.discover) {
        emitAbortOrSuccessEvent();
      }

      window.removeEventListener('beforeunload', emitAbortOrSuccessEvent);
    };

    // Only add listener and umount logic once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function onSelectResource(kind: ResourceKind) {
    setSelectedResourceKind(kind);
  }

  // nextStep takes the user to next screen and sends reporting events.
  // The prop `numToIncrement` is used in the following ways:
  //  - numToIncrement === 0, will be interpreted as user intentionally
  //    skipping the current view, to go to the next view.
  //  - numToIncrement === 1 (default), will be interpreted as user finishing
  //    the current view and is ready to go next view.
  //  - numToIncrement > 1, will be interprested as skipping some steps FOR the user
  //    eg: for Database flow, if there exists a database service, then we don't want
  //    to show the user the screen that lets them add a database service.
  function nextStep(numToIncrement = 1) {
    // This function can be used in a way that HTML event
    // get passed in which isn't a valid number.
    if (!Number.isInteger(numToIncrement)) {
      numToIncrement = 1;
    }

    const numNextSteps = numToIncrement || 1;
    const nextView = findViewAtIndex(views, currentStep + numNextSteps);
    if (nextView) {
      setCurrentStep(currentStep + numNextSteps);
      setEventState({ ...eventState, currEventName: nextView.eventName });
    }

    // Send reporting events:

    // Whenever a numToIncrement is > 1, it means some steps (after the current view)
    // are being skipped, which we should send events for.
    if (numToIncrement > 1) {
      for (let i = 1; i < numToIncrement; i++) {
        const currView = findViewAtIndex(views, currentStep + i);
        if (currView) {
          emitEvent(
            { stepStatus: DiscoverEventStatus.Skipped },
            currView.eventName
          );
        }
      }
    }

    // Emit event for the current view.
    // If user intentionally skipped the current step, then
    // skipped event will be emitted, else success.
    if (!numToIncrement) {
      emitEvent({ stepStatus: DiscoverEventStatus.Skipped });
    } else {
      emitEvent({ stepStatus: DiscoverEventStatus.Success });
    }
  }

  function prevStep() {
    const nextView = findViewAtIndex(views, currentStep - 1);

    if (nextView) {
      setCurrentStep(currentStep - 1);
    }
  }

  function updateAgentMeta(meta: AgentMeta) {
    setAgentMeta(meta);
  }

  function emitEvent(
    status: DiscoverEventStepStatus,
    eventName?: DiscoverEvent
  ) {
    const { id, currEventName, resource } = ref.current;

    userEventService.captureDiscoverEvent({
      event: eventName || currEventName,
      eventData: {
        id,
        resource,
        ...status,
      },
    });
  }

  function emitErrorEvent(errorStr = '') {
    emitEvent({
      stepStatus: DiscoverEventStatus.Error,
      stepStatusError: errorStr,
    });
  }

  // updateEventState is used when the Discover component updates in the following ways:
  //   - on initial render: sends the `start` event and will initialize the event state with
  //     the currently selected resource and create a unique ID that will tie the rest of
  //     the events that follow this `start` event.
  //   - on user updating `selectedResourceKind` (eg: server to kube)
  //     or `resourceState` (eg. postgres to mongo) which just updates the `eventState`
  function updateEventState() {
    const currEventName = findViewAtIndex(views, currentStep).eventName;
    const resource = resourceKindToEventResource(
      selectedResourceKind,
      resourceState
    );

    // Init the `eventState` and send the `started` event only once.
    if (!eventState) {
      // Generates a v4 UUID using a cryptographically secure
      // random number.
      const id = crypto.randomUUID();

      setEventState({ id, currEventName, resource });
      userEventService.captureDiscoverEvent({
        event: DiscoverEvent.Started,
        eventData: {
          id,
          stepStatus: DiscoverEventStatus.Success,
          // Started event will be the ONLY event
          // that won't expect a resource field.
          resource: '' as any,
        },
      });

      return;
    }

    setEventState({ ...eventState, currEventName, resource });
  }

  const value: DiscoverContextState<T> = {
    agentMeta,
    currentStep,
    nextStep,
    prevStep,
    onSelectResource,
    resourceState,
    selectedResource,
    selectedResourceKind,
    setResourceState,
    updateAgentMeta,
    views,
    emitErrorEvent,
    emitEvent,
    updateEventState,
    eventState,
  };

  return (
    <discoverContext.Provider value={value}>
      {props.children}
    </discoverContext.Provider>
  );
}

export function useDiscover<T = any>(): DiscoverContextState<T> {
  return useContext(discoverContext);
}

type BaseMeta = {
  // resourceName provides a consistent field to refer to for
  // the resource name since resources can refer to its name
  // by different field names.
  // Eg. used in Finish (last step) component.
  resourceName: string;
  // agentMatcherLabels are labels that will be used by the agent
  // to pick up the newly created database (looks for matching labels).
  // At least one must match.
  agentMatcherLabels: AgentLabel[];
};

// NodeMeta describes the fields for node resource
// that needs to be preserved throughout the flow.
export type NodeMeta = BaseMeta & {
  node: Node;
};

// DbMeta describes the fields for a db resource
// that needs to be preserved throughout the flow.
export type DbMeta = BaseMeta & {
  db: Database;
};

// KubeMeta describes the fields for a kube resource
// that needs to be preserved throughout the flow.
export type KubeMeta = BaseMeta & {
  kube: Kube;
};

export type AgentMeta = DbMeta | NodeMeta | KubeMeta;

export type State = ReturnType<typeof useDiscover>;
