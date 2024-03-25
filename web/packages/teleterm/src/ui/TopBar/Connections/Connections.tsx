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
import { StepComponentProps } from 'design/StepSlider';

import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { VnetSliderStep, useVnetContext } from 'teleterm/ui/Vnet';

import { useConnections } from './useConnections';
import { ConnectionsIcon } from './ConnectionsIcon/ConnectionsIcon';
import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';

export function Connections() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const connections = useConnections();
  const { status: vnetStatus } = useVnetContext();
  const isAnyConnectionActive =
    connections.isAnyConnectionActive || vnetStatus === 'running';

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => {
      const isOpened = !wasOpened;
      if (isOpened) {
        connections.updateSorting();
      }
      return isOpened;
    });
  }, [setIsPopoverOpened, connections.updateSorting]);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openConnections: togglePopover,
      }),
      [togglePopover]
    )
  );

  function activateItem(id: string): void {
    setIsPopoverOpened(false);
    connections.activateItem(id);
  }

  // TODO(ravicious): Investigate the problem with height getting temporarily reduced when switching
  // from a shorter step 1 to a taller step 2, particularly when there's an error rendered in step 2
  // that wasn't there on first render.
  //
  // It might have to do with how Popover calculates height or how StepSlider uses refs for height.
  //
  // We aim to replace the sliding animation with an expanding animation before the release, so it
  // might not be worth the effort.
  const sliderSteps = [
    (props: StepComponentProps) => (
      <Box p={2} ref={props.refCallback}>
        <KeyboardArrowsNavigation>
          <ConnectionsFilterableList
            items={connections.items}
            onActivateItem={activateItem}
            onRemoveItem={connections.removeItem}
            onDisconnectItem={connections.disconnectItem}
            slideToVnet={props.next}
          />
        </KeyboardArrowsNavigation>
      </Box>
    ),
    VnetSliderStep,
  ];

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
          <StepSlider currFlow="default" flows={{ default: sliderSteps }} />
        </Box>
      </Popover>
    </>
  );
}
