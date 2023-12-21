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

import React from 'react';

import { Text, Box } from 'design';

import cfg from 'teleport/config';

import { ActionButtons, Step, Header } from 'teleport/Discover/Shared';
import { State } from 'teleport/Discover/useDiscover';
import { generateCommand } from 'teleport/Discover/Shared/generateCommand';

interface Step {
  title: string;
  command: string;
}

const installActiveDirectorySteps: Step[] = [
  {
    title: 'Install Active Directory',
    command: generateCommand(cfg.getInstallADDSPath()),
  },
  {
    title: 'Install AD Certificate Services',
    command: generateCommand(cfg.getInstallADCSPath()),
  },
];

export function InstallActiveDirectory(props: State) {
  return (
    <Box>
      <Header>Install Active Directory</Header>

      <Text mb={4}>
        If you haven't already, install Active Directory and AD Certificate
        Services.
      </Text>

      {getSteps(installActiveDirectorySteps)}

      <ActionButtons
        onProceed={() => props.nextStep()}
        onPrev={props.prevStep}
      />
    </Box>
  );
}

function getSteps(steps: Step[]) {
  return steps.map((step, index) => (
    <Step
      key={index}
      stepNumber={index + 1}
      title={step.title}
      text={step.command}
    />
  ));
}
