/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';

import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ClusterUri } from 'teleterm/ui/uri';

import { useClusters } from './useClusters';
import { ClusterSelector } from './ClusterSelector/ClusterSelector';
import { ClustersFilterableList } from './ClustersFilterableList/ClustersFilterableList';
import ConfirmClusterChangeDialog from './ConfirmClusterChangeDialog';

export function Clusters() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const [confirmChangeTo, setConfirmChangeTo] =
    useState<ClusterUri | null>(null);
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
    if (clusters.hasPendingAccessRequest) {
      setConfirmChangeTo(clusterUri);
    } else {
      clusters.selectItem(clusterUri);
    }
  }

  function onConfirmChange(): void {
    clusters.selectItem(confirmChangeTo);
    setConfirmChangeTo(null);
    clusters.clearPendingAccessRequest();
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
      <ConfirmClusterChangeDialog
        onClose={() => setConfirmChangeTo(null)}
        onConfirm={onConfirmChange}
        confirmChangeTo={confirmChangeTo}
      />
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.levels.elevated};
`;
