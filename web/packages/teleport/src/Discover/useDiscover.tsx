/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { useCallback, useContext, useEffect, useState } from 'react';
import { useHistory, useLocation } from 'react-router';

import {
  addIndexToViews,
  findViewAtIndex,
} from 'teleport/components/Wizard/flow';
import cfg from 'teleport/config';
import type { ResourceLabel } from 'teleport/services/agents';
import type { App } from 'teleport/services/apps';
import type { Database } from 'teleport/services/databases';
import { DiscoveryConfig } from 'teleport/services/discovery';
import type {
  AwsRdsDatabase,
  IntegrationAwsOidc,
  Regions,
} from 'teleport/services/integrations';
import type { Kube } from 'teleport/services/kube';
import type { Node } from 'teleport/services/nodes';
import type {
  SamlGcpWorkforce,
  SamlIdpServiceProvider,
} from 'teleport/services/samlidp/types';
import {
  DiscoverDiscoveryConfigMethod,
  DiscoverEvent,
  DiscoverEventResource,
  DiscoverEventStatus,
  DiscoverEventStepStatus,
  DiscoverServiceDeploy,
  DiscoverServiceDeployMethod,
  DiscoverServiceDeployType,
  userEventService,
} from 'teleport/services/userEvent';

import { ServiceDeployMethod } from './Database/common';
import { ResourceViewConfig, View } from './flow';
import { viewConfigs } from './resourceViewConfigs';
import type { ResourceSpec } from './SelectResource';
import { EViewConfigs } from './types';

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
  isUpdateFlow?: boolean;
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
  serviceDeploy?: DiscoverServiceDeploy;
  discoveryConfigMethod?: DiscoverDiscoveryConfigMethod;
};

export type DiscoverUpdateProps = {
  // resourceSpec specifies ResourceSpec which should be used to
  // start a Discover flow.
  resourceSpec: ResourceSpec;
  // agentMeta includes data that will be used to prepopulate input fields
  // in the respective Discover compnents.
  agentMeta: AgentMeta;
};

type DiscoverProviderProps = {
  // mockCtx used for testing purposes.
  mockCtx?: DiscoverContextState;
  // Extra view configs that are passed in. This is used to add view configs from Enterprise.
  eViewConfigs?: EViewConfigs;
  // updateFlow holds properties used in Discover update flow.
  updateFlow?: DiscoverUpdateProps;
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
  // integration is the created aws-oidc integration
  integration: IntegrationAwsOidc;
};

const discoverContext = React.createContext<DiscoverContextState>(null);

export function DiscoverProvider({
  mockCtx,
  children,
  eViewConfigs = [],
  updateFlow,
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

      let serviceDeploy: DiscoverServiceDeploy;
      if (event === DiscoverEvent.DeployService) {
        if (custom?.serviceDeploy) {
          serviceDeploy = custom.serviceDeploy;
        } else {
          serviceDeploy = {
            method: DiscoverServiceDeployMethod.Unspecified,
            type: DiscoverServiceDeployType.Unspecified,
          };
        }
      }

      let discoveryConfigMethod: DiscoverDiscoveryConfigMethod;
      if (event === DiscoverEvent.CreateDiscoveryConfig) {
        if (custom?.discoveryConfigMethod) {
          discoveryConfigMethod = custom.discoveryConfigMethod;
        } else {
          discoveryConfigMethod = DiscoverDiscoveryConfigMethod.Unspecified;
        }
      }

      userEventService.captureDiscoverEvent({
        event,
        eventData: {
          id: id || custom?.id,
          resource: custom?.eventResourceName || resourceSpec?.event,
          autoDiscoverResourcesCount: custom?.autoDiscoverResourcesCount,
          selectedResourcesCount: custom?.selectedResourcesCount,
          serviceDeploy,
          discoveryConfigMethod,
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

  // trigger update Discover flow.
  useEffect(() => {
    if (updateFlow && updateFlow.agentMeta) {
      updateAgentMeta(updateFlow.agentMeta);
      onSelectResource(updateFlow.resourceSpec);
    }
  }, [updateFlow]);

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
    const { discover, integration } = location.state;

    updateAgentMeta({ awsIntegration: integration });

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
    let indexedViews: View[] = [];
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
        serviceDeploy: {
          method: DiscoverServiceDeployMethod.Unspecified,
          type: DiscoverServiceDeployType.Unspecified,
        },
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
    isUpdateFlow: !!updateFlow,
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
  /**
   * resourceName provides a consistent field to refer to since
   * different resources can refer to its name by different field names.
   * Eg. used in Finish (last step) component.
   * This field is set when user has finished enrolling a resource.
   */
  resourceName?: string;
  /**
   * agentMatcherLabels are labels (defined in the enrolled resource)
   * that are suggested to the user to be used as label matcher for
   * an agent.
   *
   * This field is set when user has finished enrolling a resource.
   */
  agentMatcherLabels?: ResourceLabel[];

  /**
   * awsIntegration is set to the selected AWS integration.
   * This field is set when a user wants to enroll AWS resources.
   */
  awsIntegration?: IntegrationAwsOidc;
  /**
   * awsRegion is set to the selected AWS region.
   * This field is set when a user wants to enroll AWS resources.
   */
  awsRegion?: Regions;
  /**
   * If this field is defined, then user opted for auto discovery.
   * Auto discover will automatically identify and register resources
   * in customers infrastructure such as Kubernetes clusters or databases hosted
   * on cloud platforms like AWS, Azure, etc.
   */
  autoDiscovery?: AutoDiscovery;
  /**
   * If this field is defined, it means the user selected a specific vpc ID.
   * Not all flows will allow a user to select a vpc ID.
   */
  awsVpcId?: string;
};

export type AutoDiscovery = {
  config?: DiscoveryConfig;
  // requiredVpcsAndSubnets is a map of required vpcs for auto discovery.
  // If this is empty, then a user can skip deploying db agents.
  // If >0, auto discovery requires deploying db agents.
  requiredVpcsAndSubnets?: Record<string, string[]>;
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
  db?: Database;
  selectedAwsRdsDb?: AwsRdsDatabase;
  /**
   * serviceDeployedMethod flag will be undefined if user skipped
   * deploying service (service already existed).
   */
  serviceDeployedMethod?: ServiceDeployMethod;
};

// KubeMeta describes the fields for a kube resource
// that needs to be preserved throughout the flow.
export type KubeMeta = BaseMeta & {
  kube: Kube;
};

/**
 * EksMeta describes the fields for a kube resource
 * that needs to be preserved throughout the flow.
 */
export type EksMeta = BaseMeta & {
  kube: Kube;
};

/**
 * AppMeta describes the fields for a app resource
 * that needs to be preserved throughout the flow.
 */
export type AppMeta = BaseMeta & {
  app: App;
};

/**
 * SamlMeta describes the fields for SAML IdP
 * service provider resource that needs to be
 * preserved throughout the flow.
 */
export type SamlMeta = BaseMeta & {
  samlGeneric?: SamlIdpServiceProvider;
  samlGcpWorkforce?: SamlGcpWorkforce;
};

export type AgentMeta =
  | DbMeta
  | NodeMeta
  | KubeMeta
  | EksMeta
  | SamlMeta
  | AppMeta;

export type State = ReturnType<typeof useDiscover>;
