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

import { useState, useEffect } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import type {
  JoinMethod,
  JoinRole,
  JoinToken,
  JoinRule,
} from 'teleport/services/joinToken';
import { DiscoverContext } from './discoverContext';

export function useDiscover(ctx: DiscoverContext) {
  const { attempt, run } = useAttempt('');
  const { attempt: initAttempt, run: initRun } = useAttempt('processing');

  const [joinToken, setJoinToken] = useState<JoinToken>();
  const [currentStep, setCurrentStep] = useState<AgentStep>(0);
  const [selectedAgentKind, setSelectedAgentKind] = useState<AgentKind>();
  const [agentMeta, setAgentMeta] = useState<AgentMeta>();

  useEffect(() => {
    initRun(() => ctx.init());
  }, []);

  function onSelectResource(kind: AgentKind) {
    setSelectedAgentKind(kind);
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
    ctx.logout();
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
    initAttempt,
    username: ctx.username,
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

// AgentStep defines the order of steps in `connecting a agent (resource)`
// that all agent kinds should share.
//
// The numerical enum value is used to determine which step the user is currently in,
// which is also used as the `index value` to access array's values
// for `agentStepTitles` and `agentViews`.
export enum AgentStep {
  Select = 0,
  Setup,
  RoleConfig,
  TestConnection,
}

// NodeMeta describes the fields that may be provided or required by user
// when connecting a node.
type NodeMeta = {
  awsAccountId?: string;
  awsArn?: string;
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
