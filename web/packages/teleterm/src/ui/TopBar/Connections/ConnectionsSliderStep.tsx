/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Box } from 'design';
import { StepComponentProps } from 'design/StepSlider';

import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

import { ConnectionsFilterableList } from './ConnectionsFilterableList/ConnectionsFilterableList';
import { useConnections } from './useConnections';

export const ConnectionsSliderStep = (
  props: StepComponentProps & { activateItem: (id: string) => void }
) => {
  const connections = useConnections();

  return (
    <Box p={2} ref={props.refCallback}>
      <KeyboardArrowsNavigation>
        <ConnectionsFilterableList
          items={connections.items}
          onActivateItem={props.activateItem}
          onRemoveItem={connections.removeItem}
          onDisconnectItem={connections.disconnectItem}
          slideToVnet={props.next}
        />
      </KeyboardArrowsNavigation>
    </Box>
  );
};
