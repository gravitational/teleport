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

import { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import { Box, StepSlider } from 'design';

import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { VnetSliderStep, useVnetContext } from 'teleterm/ui/Vnet';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsSliderStep } from './ConnectionsSliderStep';

export function Connections() {
  const { connectionTracker } = useAppContext();
  connectionTracker.useState();
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const { status: vnetStatus } = useVnetContext();
  const isAnyConnectionActive =
    connectionTracker.getConnections().some(c => c.connected) ||
    vnetStatus === 'running';

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, []);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openConnections: togglePopover,
      }),
      [togglePopover]
    )
  );

  function closeConnectionList(): void {
    setIsPopoverOpened(false);
  }

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
        onClick={togglePopover}
        ref={iconRef}
      />
      <Popover
        open={isPopoverOpened}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        {/*
          324px matches the total width when the outer div inside Popover used to have 12px of
          padding (so 24px on both sides) and ConnectionsFilterableList had 300px of width.
        */}
        <Box width="324px" bg="levels.elevated">
          <StepSlider
            currFlow="default"
            flows={stepSliderFlows}
            // The rest of the props is spread to each individual step component.
            closeConnectionList={closeConnectionList}
          />
        </Box>
      </Popover>
    </>
  );
}

const stepSliderFlows = { default: [ConnectionsSliderStep, VnetSliderStep] };
