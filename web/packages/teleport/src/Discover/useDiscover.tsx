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

import React, { useContext, useState, useEffect, useCallback } from 'react';
import { useHistory } from 'react-router';

import {
  DiscoverEventStatus,
  DiscoverEventStepStatus,
  DiscoverEvent,
  DiscoverEventResource,
  userEventService,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import {
  addIndexToViews,
  findViewAtIndex,
  ResourceViewConfig,
  View,
} from './flow';
import { viewConfigs } from './resourceViewConfigs';

import type { Node } from 'teleport/services/nodes';
import type { Kube } from 'teleport/services/kube';
import type { Database } from 'teleport/services/databases';
import type { AgentLabel } from 'teleport/services/agents';
import type { ResourceSpec } from './SelectResource';

interface DiscoverContextState<T = any> {
  agentMeta: AgentMeta;
  currentStep: number;
  nextStep: (count?: number) => void;
  prevStep: () => void;
  onSelectResource: (resource: ResourceSpec) => void;
  resourceSpec: ResourceSpec;
  viewConfig: ResourceViewConfig<T>;
  indexedViews: View[];
  setResourceSpec: (value: T) => void;
  updateAgentMeta: (meta: AgentMeta) => void;
  emitErrorEvent(errorStr: string): void;
  emitEvent(status: DiscoverEventStepStatus, custom?: CustomEventInput): void;
  eventState: EventState;
}

type EventState = {
  id: string;
  currEventName: DiscoverEvent;
  manuallyEmitSuccessEvent: boolean;
};

type CustomEventInput = {
  eventName?: DiscoverEvent;
  eventResourceName?: DiscoverEventResource;
  autoDiscoverResourcesCount?: number;
};

const discoverContext = React.createContext<DiscoverContextState>(null);

export function DiscoverProvider(props: React.PropsWithChildren<unknown>) {
  const history = useHistory();

  const [currentStep, setCurrentStep] = useState(0);
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();
  const [resourceSpec, setResourceSpec] = useState<ResourceSpec>();
  const [viewConfig, setViewConfig] = useState<ResourceViewConfig>();
  const [eventState, setEventState] = useState<EventState>();

  // indexedViews contains views of the selected resource where
  // each view has been assigned an index value.
  const [indexedViews, setIndexedViews] = useState<View[]>([]);

  const emitEvent = useCallback(
    (status: DiscoverEventStepStatus, custom?: CustomEventInput) => {
      const { id, currEventName } = eventState;

      userEventService.captureDiscoverEvent({
        event: custom?.eventName || currEventName,
        eventData: {
          id,
          resource: custom?.eventResourceName || resourceSpec?.event,
          autoDiscoverResourcesCount: custom?.autoDiscoverResourcesCount,
          ...status,
        },
      });
    },
    [eventState, resourceSpec]
  );

  useEffect(() => {
    const emitAbortOrSuccessEvent = () => {
      if (eventState.currEventName === DiscoverEvent.Completed) {
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
  }, [eventState, history.location.pathname, emitEvent]);

  useEffect(() => {
    initEventState();
  }, []);

  function initEventState() {
    // Generates a v4 UUID using a cryptographically secure
    // random number.
    const id = crypto.randomUUID();

    setEventState({
      id,
      currEventName: DiscoverEvent.Started,
      manuallyEmitSuccessEvent: null,
    });
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
  }

  // onSelectResources initializes all the required
  // variables needed to start a guided flow.
  function onSelectResource(resource: ResourceSpec) {
    // We still want to emit an event if user clicked on
    // unguided links to gather data on which unguided resource
    // is most popular.
    if (resource.unguidedLink) {
      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.ResourceSelection,
          eventResourceName: resource.event,
        }
      );
      return;
    }

    // Process each view and assign each with an index number.
    const currCfg = viewConfigs.find(r => r.kind === resource.kind);
    let indexedViews = [];
    if (typeof currCfg.views === 'function') {
      indexedViews = addIndexToViews(currCfg.views(resource));
    } else {
      indexedViews = addIndexToViews(currCfg.views);
    }

    // Find the first view to update the event state.
    const { eventName, manuallyEmitSuccessEvent } = findViewAtIndex(
      indexedViews,
      currentStep
    );
    // At this point it's considered the user has
    // successfully selected a resource, so we send an event.
    emitEvent(
      { stepStatus: DiscoverEventStatus.Success },
      {
        eventName: DiscoverEvent.ResourceSelection,
        eventResourceName: resource.event,
      }
    );

    // Init all required states to start the flow.
    setEventState({
      ...eventState,
      currEventName: eventName,
      manuallyEmitSuccessEvent,
    });
    setViewConfig(currCfg);
    setIndexedViews(indexedViews);
    setResourceSpec(resource);
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
    const nextView = findViewAtIndex(indexedViews, currentStep + numNextSteps);
    if (nextView) {
      setCurrentStep(currentStep + numNextSteps);
      setEventState({
        ...eventState,
        currEventName: nextView.eventName,
        manuallyEmitSuccessEvent: nextView.manuallyEmitSuccessEvent,
      });
    }

    // Send reporting events:

    // Emit event for the current view.
    // If user intentionally skipped the current step, then
    // skipped event will be emitted, else success.
    if (!numToIncrement) {
      emitEvent({ stepStatus: DiscoverEventStatus.Skipped });
    } else if (!eventState.manuallyEmitSuccessEvent) {
      emitEvent({ stepStatus: DiscoverEventStatus.Success });
    }

    // Whenever a numToIncrement is > 1, it means some steps (after the current view)
    // are being skipped, which we should send events for.
    if (numToIncrement > 1) {
      for (let i = 1; i < numToIncrement; i++) {
        const currView = findViewAtIndex(indexedViews, currentStep + i);
        if (currView) {
          emitEvent(
            { stepStatus: DiscoverEventStatus.Skipped },
            { eventName: currView.eventName }
          );
        }
      }
    }
  }

  function prevStep() {
    if (currentStep === 0) {
      // Emit abort since we are starting over with resource selection.
      emitEvent({ stepStatus: DiscoverEventStatus.Aborted });
      initEventState();
      setViewConfig(null);
      setResourceSpec(null);
      setIndexedViews([]);
      return;
    }

    const updatedCurrentStep = currentStep - 1;
    const nextView = findViewAtIndex(indexedViews, updatedCurrentStep);
    if (nextView) {
      setCurrentStep(updatedCurrentStep);
    }
  }

  function updateAgentMeta(meta: AgentMeta) {
    setAgentMeta(meta);
  }

  function emitErrorEvent(errorStr = '') {
    emitEvent({
      stepStatus: DiscoverEventStatus.Error,
      stepStatusError: errorStr,
    });
  }

  const value: DiscoverContextState = {
    agentMeta,
    currentStep,
    nextStep,
    prevStep,
    onSelectResource,
    resourceSpec,
    viewConfig,
    setResourceSpec,
    updateAgentMeta,
    indexedViews,
    emitErrorEvent,
    emitEvent,
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
