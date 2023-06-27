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
import { useHistory, useLocation } from 'react-router';

import {
  DiscoverEventStatus,
  DiscoverEventStepStatus,
  DiscoverEvent,
  DiscoverEventResource,
  userEventService,
  DiscoverServiceDeployMethod,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import {
  addIndexToViews,
  findViewAtIndex,
  ResourceViewConfig,
  View,
} from './flow';
import { viewConfigs } from './resourceViewConfigs';
import { EViewConfigs } from './types';

import type { Node } from 'teleport/services/nodes';
import type { Kube } from 'teleport/services/kube';
import type { Database } from 'teleport/services/databases';
import type { AgentLabel } from 'teleport/services/agents';
import type { ResourceSpec } from './SelectResource';

export interface DiscoverContextState<T = any> {
  agentMeta: AgentMeta;
  currentStep: number;
  nextStep: (count?: number) => void;
  prevStep: () => void;
  onSelectResource: (resource: ResourceSpec) => void;
  exitFlow: () => void;
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
  id?: string;
  eventName?: DiscoverEvent;
  eventResourceName?: DiscoverEventResource;
  autoDiscoverResourcesCount?: number;
  selectedResourcesCount?: number;
  serviceDeployedMethod?: DiscoverServiceDeployMethod;
};

type DiscoverProviderProps = {
  // mockCtx used for testing purposes.
  mockCtx?: DiscoverContextState;
  // Extra view configs that are passed in. This is used to add view configs from Enterprise.
  eViewConfigs?: EViewConfigs;
};

// DiscoverUrlLocationState define fields to preserve state between
// react routes (eg. in RDS database flow, it is required of user
// to create a AWS OIDC integration which requires changing route
// and then coming back to resume the flow.)
export type DiscoverUrlLocationState = {
  // discover contains the fields necessary to be able to resume
  // the flow from where user left off.
  discover: {
    eventState: EventState;
    resourceSpec: ResourceSpec;
    currentStep: number;
  };
  // integrationName is the name of the created integration
  // resource name (eg: integration subkind "aws-oidc")
  integrationName: string;
};

const discoverContext = React.createContext<DiscoverContextState>(null);

export function DiscoverProvider({
  mockCtx,
  children,
  eViewConfigs = [],
}: React.PropsWithChildren<DiscoverProviderProps>) {
  const history = useHistory();
  const location = useLocation<DiscoverUrlLocationState>();

  const [currentStep, setCurrentStep] = useState(0);
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();
  const [resourceSpec, setResourceSpec] = useState<ResourceSpec>();
  const [viewConfig, setViewConfig] = useState<ResourceViewConfig>();
  const [eventState, setEventState] = useState<EventState>({} as any);

  // indexedViews contains views of the selected resource where
  // each view has been assigned an index value.
  const [indexedViews, setIndexedViews] = useState<View[]>([]);

  const emitEvent = useCallback(
    (status: DiscoverEventStepStatus, custom?: CustomEventInput) => {
      const { id, currEventName } = eventState;

      const event = custom?.eventName || currEventName;
      const isDeployEvent = event === DiscoverEvent.DeployService;
      const deployedMethod =
        custom?.serviceDeployedMethod ||
        DiscoverServiceDeployMethod.Unspecified;

      userEventService.captureDiscoverEvent({
        event,
        eventData: {
          id: id || custom.id,
          resource: custom?.eventResourceName || resourceSpec?.event,
          autoDiscoverResourcesCount: custom?.autoDiscoverResourcesCount,
          selectedResourcesCount: custom?.selectedResourcesCount,
          serviceDeployedMethod: isDeployEvent ? deployedMethod : undefined,
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
    if (location.state?.discover) {
      resumeDiscoverFlow();
    } else {
      initEventState();
    }
  }, []);

  function initEventState() {
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

  // If a location.state.discover was provided, that means the user is
  // coming back from another location to resume the flow.
  // Users will resume at the step that is +1 from the step they left from.
  //
  // Example (only applies to AWS RDS & Aurora resources):
  // A user can leave from route `web/discover/<Connect AWS Account>`
  // to `web/integrations/enroll/<Create AWS OIDC Integration>` then
  // come back to resume flow at `web/discover/<Enroll RDS Database>`
  //
  // Resuming flow at `Enroll RDS Database` means the user has
  // successfully finished the prior `Connect AWS Account` step,
  // so we emit a success event for that step.
  //
  // The location.state.discover should contain all the state that allows
  // the user to resume from where they left of.
  function resumeDiscoverFlow() {
    const { discover, integrationName } = location.state;

    updateAgentMeta({ integrationName } as DbMeta);

    startDiscoverFlow(
      discover.resourceSpec,
      discover.eventState,
      discover.currentStep + 1
    );

    emitEvent(
      { stepStatus: DiscoverEventStatus.Success },
      {
        eventName: discover.eventState.currEventName,
        eventResourceName: discover.resourceSpec.event,
        id: discover.eventState.id,
      }
    );
  }

  // onSelectResources inits states, starts flow, and
  // emits events.
  function onSelectResource(resource: ResourceSpec) {
    // We still want to emit an event if user clicked on an
    // unguided link to gather data on which unguided resource
    // is most popular.
    if (resource.unguidedLink || resource.isDialog) {
      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.ResourceSelection,
          eventResourceName: resource.event,
        }
      );
      return;
    }

    startDiscoverFlow(resource, eventState);

    // At this point it's considered the user has
    // successfully selected a resource, so we send an event
    // for it.
    emitEvent(
      { stepStatus: DiscoverEventStatus.Success },
      {
        eventName: DiscoverEvent.ResourceSelection,
        eventResourceName: resource.event,
      }
    );
  }

  // startDiscoverFlow sets all the required states
  // that will begin the flow.
  function startDiscoverFlow(
    resource: ResourceSpec,
    initEventState: EventState,
    targetViewIndex = 0
  ) {
    // Process each view and assign each with an index number.
    const currCfg = [...viewConfigs, ...eViewConfigs].find(
      r => r.kind === resource.kind
    );
    let indexedViews = [];
    if (typeof currCfg.views === 'function') {
      indexedViews = addIndexToViews(currCfg.views(resource));
    } else {
      indexedViews = addIndexToViews(currCfg.views);
    }

    // Find the target view to update the event state.
    const { eventName, manuallyEmitSuccessEvent } = findViewAtIndex(
      indexedViews,
      targetViewIndex
    );

    // Init all required states to start the flow.
    setEventState({
      ...initEventState,
      currEventName: eventName,
      manuallyEmitSuccessEvent,
    });
    setViewConfig(currCfg);
    setIndexedViews(indexedViews);
    setResourceSpec(resource);
    setCurrentStep(targetViewIndex);
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
      // TODO(lisa): Currently the RDS enroll screen only allows
      // user to select one RDS database to enroll so we hard code
      // it for now.
      if (eventState.currEventName === DiscoverEvent.DatabaseRDSEnrollEvent) {
        emitEvent(
          { stepStatus: DiscoverEventStatus.Success },
          { selectedResourcesCount: 1 }
        );
      } else {
        emitEvent({ stepStatus: DiscoverEventStatus.Success });
      }
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
      exitFlow();
      return;
    }

    const updatedCurrentStep = currentStep - 1;
    const nextView = findViewAtIndex(indexedViews, updatedCurrentStep);
    if (nextView) {
      setCurrentStep(updatedCurrentStep);
    }
  }

  function exitFlow() {
    initEventState();
    setViewConfig(null);
    setResourceSpec(null);
    setIndexedViews([]);
  }

  function updateAgentMeta(meta: AgentMeta) {
    setAgentMeta(meta);
  }

  function emitErrorEvent(errorStr = '') {
    emitEvent(
      {
        stepStatus: DiscoverEventStatus.Error,
        stepStatusError: errorStr,
      },
      {
        autoDiscoverResourcesCount: 0,
        selectedResourcesCount: 0,
        serviceDeployedMethod: DiscoverServiceDeployMethod.Unspecified,
      }
    );
  }

  const value: DiscoverContextState = {
    agentMeta,
    currentStep,
    nextStep,
    prevStep,
    onSelectResource,
    exitFlow,
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
    <discoverContext.Provider value={mockCtx || value}>
      {children}
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
  // TODO(lisa): when we can enroll multiple RDS's, turn this into an array?
  // The enroll event expects num count of enrolled RDS's, update accordingly.
  db: Database;
  integrationName?: string;
};

// KubeMeta describes the fields for a kube resource
// that needs to be preserved throughout the flow.
export type KubeMeta = BaseMeta & {
  kube: Kube;
};

export type AgentMeta = DbMeta | NodeMeta | KubeMeta;

export type State = ReturnType<typeof useDiscover>;
