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

import { useConnectionsContext } from './connectionsContext';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsSliderStep } from './ConnectionsSliderStep';

export function Connections() {
  const { connectionTracker } = useAppContext();
  connectionTracker.useState();
  const iconRef = useRef(undefined);
  const { isOpen, toggle, close } = useConnectionsContext();
  const isAnyConnectionActive = connectionTracker
    .getConnections()
    .some(c => c.connected);
  const status = isAnyConnectionActive ? 'on' : 'off';

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openConnections: toggle,
      }),
      [toggle]
    )
  );

  return (
    <>
      <ConnectionsIcon status={status} onClick={toggle} ref={iconRef} />
      <Popover
        open={isOpen}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={close}
      >
        <Box width="396px" bg="levels.elevated">
          <StepSlider
            tDuration={250}
            currFlow="default"
            flows={stepSliderFlows}
          />
        </Box>
      </Popover>
    </>
  );
}

const stepSliderFlows = { default: [ConnectionsSliderStep] };
