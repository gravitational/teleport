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

import { useMemo, useState } from 'react';

import { useLocation } from 'react-router';

import session from 'teleport/services/websession';
import useMain, { UseMainConfig } from 'teleport/Main/useMain';

import { ResourceKind } from 'teleport/Discover/Shared';

import { addIndexToViews, findViewAtIndex, View } from './flow';

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

export function useDiscover(config: UseMainConfig) {
  const initState = useMain(config);
  const location = useLocation<{ entity: string }>();

  const [currentStep, setCurrentStep] = useState(0);
  const [selectedResourceKind, setSelectedResourceKind] =
    useState<ResourceKind>(getKindFromString(location?.state?.entity));
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();

  const selectedResource = resources.find(r => r.kind === selectedResourceKind);
  const views = useMemo<View[]>(
    () => addIndexToViews(selectedResource.views),
    [selectedResource.views]
  );

  function onSelectResource(kind: ResourceKind) {
    setSelectedResourceKind(kind);
  }

  // nextStep takes the user to the next screen.
  // The prop `numToIncrement` is used (>1) when we want to
  // skip some number of steps.
  // eg: particularly for Database flow, if there exists a
  // database service, then we don't want to show the user
  // the screen that lets them add a database server.
  function nextStep(numToIncrement = 1) {
    const nextView = findViewAtIndex(views, currentStep + numToIncrement);

    if (nextView) {
      setCurrentStep(currentStep + numToIncrement);
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

  function logout() {
    session.logout();
  }

  return {
    agentMeta,
    alerts: initState.alerts,
    currentStep,
    customBanners: initState.customBanners,
    dismissAlert: initState.dismissAlert,
    initAttempt: { status: initState.status, statusText: initState.statusText },
    logout,
    nextStep,
    prevStep,
    onSelectResource,
    selectedResource,
    selectedResourceKind,
    updateAgentMeta,
    views,
  };
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
