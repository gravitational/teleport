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
import styled from 'styled-components';

import { Box } from 'design';
import Popover from 'design/Popover';

import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { ClusterUri } from 'teleterm/ui/uri';

import { ClusterSelector } from './ClusterSelector/ClusterSelector';
import { ClustersFilterableList } from './ClustersFilterableList/ClustersFilterableList';
import { useClusters } from './useClusters';

export function Clusters() {
  const iconRef = useRef(undefined);
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const clusters = useClusters();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, [setIsPopoverOpened]);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openClusters: togglePopover,
      }),
      [togglePopover]
    )
  );

  function selectItem(clusterUri: ClusterUri): void {
    setIsPopoverOpened(false);
    clusters.selectItem(clusterUri);
  }

  if (!clusters.hasLeaves) {
    return null;
  }

  return (
    <>
      <ClusterSelector
        clusterName={clusters.selectedItem?.name}
        onClick={togglePopover}
        isOpened={isPopoverOpened}
        ref={iconRef}
      />
      <Popover
        open={isPopoverOpened}
        anchorEl={iconRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        <Container p="12px">
          <KeyboardArrowsNavigation>
            <ClustersFilterableList
              items={clusters.items}
              onSelectItem={selectItem}
              selectedItem={clusters.selectedItem}
            />
          </KeyboardArrowsNavigation>
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.levels.elevated};
`;
