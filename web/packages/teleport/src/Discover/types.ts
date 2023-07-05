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
import { ResourceViewConfig } from './flow';

export type AgentStepProps = {
  // agentMeta describes fields specific to an agent kind.
  agentMeta?: State['agentMeta'];
  // updateAgentMeta updates the data specific to agent kinds
  // as needed as we move through the step.
  updateAgentMeta?: State['updateAgentMeta'];
  // nextStep increments the `currentStep` to go to the next step.
  nextStep?: State['nextStep'];
  // prevStep decrements the `currentStep` to go to the prev step.
  prevStep?: State['prevStep'];
  resourceSpec?: State['resourceSpec'];
};

export type AgentStepComponent = (props: AgentStepProps) => JSX.Element;

/** EViewConfigs are enterprise-only view configs to add to Discover that arent defined in `Discover/resourceViewConfigs.ts`. */
export type EViewConfigs = ResourceViewConfig[];
