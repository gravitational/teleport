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

import { ResourceViewConfig } from './flow';
import { State } from './useDiscover';

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
