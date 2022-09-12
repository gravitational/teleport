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

import { useState } from 'react';
import { useLocation } from 'react-router';

import TeleportContext from 'teleport/teleportContext';
import session from 'teleport/services/websession';
import useMain from 'teleport/Main/useMain';

import type { Node } from 'teleport/services/nodes';
import type { Feature } from 'teleport/types';

export function useDiscover(ctx: TeleportContext, features: Feature[]) {
  const initState = useMain(features);
  const location: Loc = useLocation();

  const [currentStep, setCurrentStep] = useState(0);
  const [selectedAgentKind, setSelectedAgentKind] = useState<AgentKind>(
    location?.state?.entity || 'server'
  );
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();

  function onSelectResource(kind: AgentKind) {
    setSelectedAgentKind(kind);
  }

  function nextStep() {
    setCurrentStep(currentStep + 1);
  }

  function updateAgentMeta(meta: AgentMeta) {
    setAgentMeta(meta);
  }

  function logout() {
    session.logout();
  }

  return {
    alerts: initState.alerts,
    customBanners: initState.customBanners,
    dismissAlert: initState.dismissAlert,
    initAttempt: { status: initState.status, statusText: initState.statusText },
    userMenuItems: ctx.storeNav.getTopMenuItems(),
    username: ctx.storeUser.getUsername(),
    currentStep,
    logout,
    // Rest of the exported fields are used to prop drill
    // to Step 2+ components.
    onSelectResource,
    selectedAgentKind,
    agentMeta,
    updateAgentMeta,
    nextStep,
  };
}

type Loc = {
  state: {
    entity: AgentKind;
  };
};

type BaseMeta = {
  resourceName: string;
};

// NodeMeta describes the fields for node resource
// that needs to be preserved throughout the flow.
export type NodeMeta = BaseMeta & {
  node: Node;
};

// AppMeta describes the fields that may be provided or required by user
// when connecting a app.
type AppMeta = BaseMeta & {
  name: string;
  publicAddr: string;
};

export type AgentMeta = AppMeta | NodeMeta;

export type AgentKind =
  | 'application'
  | 'database'
  | 'desktop'
  | 'kubernetes'
  | 'server';

export type State = ReturnType<typeof useDiscover>;
