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
import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContext from 'teleport/teleportContext';
import session from 'teleport/services/websession';
import useMain from 'teleport/Main/useMain';

import type { Node } from 'teleport/services/nodes';

import type {
  JoinMethod,
  JoinRole,
  JoinToken,
  JoinRule,
} from 'teleport/services/joinToken';
import type { Feature } from 'teleport/types';

export function useDiscover(ctx: TeleportContext, features: Feature[]) {
  const initState = useMain(features);
  const { attempt, run } = useAttempt('');

  const [joinToken, setJoinToken] = useState<JoinToken>();
  const [currentStep, setCurrentStep] = useState(0);
  const [selectedAgentKind, setSelectedAgentKind] = useState<AgentKind>();
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();

  function onSelectResource(kind: AgentKind = 'node') {
    // TODO: hard coded for now for sake of testing the flow.
    setSelectedAgentKind(kind);
    nextStep();
  }

  function nextStep() {
    setCurrentStep(currentStep + 1);
  }

  function prevStep() {
    setCurrentStep(currentStep - 1);
  }

  function updateAgentMeta(meta: AgentMeta) {
    setAgentMeta(meta);
  }

  function logout() {
    session.logout();
  }

  function createJoinToken(method: JoinMethod = 'token', rules?: JoinRule[]) {
    let systemRole: JoinRole;
    switch (selectedAgentKind) {
      case 'app':
        systemRole = 'App';
        break;
      case 'db':
        systemRole = 'Db';
        break;
      case 'desktop':
        systemRole = 'WindowsDesktop';
        break;
      case 'kube':
        systemRole = 'Kube';
        break;
      case 'node':
        systemRole = 'Node';
        break;
      default:
        console;
    }

    run(() =>
      ctx.joinTokenService
        .fetchJoinToken([systemRole], method, rules)
        .then(setJoinToken)
    );
  }

  return {
    initAttempt: { status: initState.status, statusText: initState.statusText },
    userMenuItems: ctx.storeNav.getTopMenuItems(),
    username: ctx.storeUser.getUsername(),
    currentStep,
    selectedAgentKind,
    logout,
    onSelectResource,
    // Rest of the exported fields are used to prop drill
    // to Step 2+ components.
    attempt,
    joinToken,
    agentMeta,
    updateAgentMeta,
    nextStep,
    prevStep,
    createJoinToken,
  };
}

// NodeMeta describes the fields for node resource
// that needs to be preserved throughout the flow.
export type NodeMeta = {
  node: Node;
};

// AppMeta describes the fields that may be provided or required by user
// when connecting a app.
type AppMeta = {
  name: string;
  publicAddr: string;
};

export type AgentMeta = AppMeta | NodeMeta;

export type AgentKind = 'app' | 'db' | 'desktop' | 'kube' | 'node';

export type State = ReturnType<typeof useDiscover>;
