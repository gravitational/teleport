/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useRef } from 'react';

import { Box } from 'design';
import Popover from 'design/Popover';

import { Status } from 'teleterm/ui/TopBar/Connections/ConnectionsIcon/ConnectionsIconStatusIndicator';
import { useVnetContext, VnetPanel } from 'teleterm/ui/Vnet';

import { VnetIcon } from './VnetIcon';

/**
 * Vnet is the VNet icon and its panel, shown in the top bar. The panel opens in
 * a popover anchored to the icon.
 */
export function Vnet() {
  const iconRef = useRef(undefined);
  const {
    isSupported,
    isPanelOpen,
    togglePanel,
    closePanel,
    status: vnetStatus,
    showDiagWarningIndicator,
  } = useVnetContext();

  if (!isSupported) {
    return null;
  }

  const status: Status = showDiagWarningIndicator
    ? 'warning'
    : vnetStatus.value === 'running'
      ? 'on'
      : 'off';

  return (
    <>
      <VnetIcon status={status} onClick={togglePanel} ref={iconRef} />
      <Popover
        open={isPanelOpen}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={closePanel}
      >
        {/*
          It needs to be wide enough for the diag warning to not be squished too much.
        */}
        <Box width="396px" bg="levels.elevated">
          <VnetPanel />
        </Box>
      </Popover>
    </>
  );
}
