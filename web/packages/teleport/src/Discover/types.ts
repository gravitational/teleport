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

import { State } from './useDiscover';

export type AgentStepProps = {
  // attempt defines fetch attempt states when we make api calls.
  attempt: State['attempt'];
  // joinToken defines fields related to a fetched token.
  joinToken: State['joinToken'];
  // agentMeta describes fields specific to an agent kind.
  agentMeta: State['agentMeta'];
  // updateAgentMeta updates the data specific to agent kinds
  // as needed as we move through the step.
  updateAgentMeta: State['updateAgentMeta'];
  // nextStep increments the `currentStep` to go to the next step.
  nextStep: State['nextStep'];
  // prevStep decrements the `currentStep` to go to the prev step.
  // TODO (anyone): I think we established there will be no previous
  // button, but I don't remember.
  prevStep: State['prevStep'];
  // createJoinToken makes a fetch api call to get a joinToken.
  createJoinToken: State['createJoinToken'];
};

export type AgentStepComponent = (props: AgentStepProps) => JSX.Element;
