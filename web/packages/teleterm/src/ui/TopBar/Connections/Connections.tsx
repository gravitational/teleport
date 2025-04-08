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

import { useMemo, useRef } from 'react';

import { Box, StepSlider } from 'design';
import Popover from 'design/Popover';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { useVnetContext, VnetSliderStep } from 'teleterm/ui/Vnet';

import { Step, useConnectionsContext } from './connectionsContext';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsSliderStep } from './ConnectionsSliderStep';

export function Connections() {
  const { connectionTracker } = useAppContext();
  connectionTracker.useState();
  const iconRef = useRef();
  const { isOpen, toggle, close, stepToOpen } = useConnectionsContext();
  const { status: vnetStatus } = useVnetContext();
  const isAnyConnectionActive =
    connectionTracker.getConnections().some(c => c.connected) ||
    vnetStatus.value === 'running';

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openConnections: toggle,
      }),
      [toggle]
    )
  );

  // TODO(ravicious): Investigate the problem with height getting temporarily reduced when switching
  // from a shorter step 1 to a taller step 2, particularly when there's an error rendered in step 2
  // that wasn't there on first render.
  //
  // It might have to do with how Popover calculates height or how StepSlider uses refs for height.
  //
  // We aim to replace the sliding animation with an expanding animation before the release, so it
  // might not be worth the effort.

  return (
    <>
      <ConnectionsIcon
        isAnyConnectionActive={isAnyConnectionActive}
        onClick={toggle}
        ref={iconRef}
      />
      <Popover
        open={isOpen}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={close}
      >
        {/*
          324px matches the total width when the outer div inside Popover used to have 12px of
          padding (so 24px on both sides) and ConnectionsFilterableList had 300px of width.
        */}
        <Box width="324px" bg="levels.elevated">
          <StepSlider
            tDuration={250}
            currFlow="default"
            flows={stepSliderFlows}
            defaultStepIndex={stepToIndex(stepToOpen)}
          />
        </Box>
      </Popover>
    </>
  );
}

const stepSliderFlows = { default: [ConnectionsSliderStep, VnetSliderStep] };

const stepToIndex = (step: Step): number => {
  switch (step) {
    case 'connections':
      return 0;
    case 'vnet':
      return 1;
    default:
      step satisfies never;
      return 0;
  }
};
