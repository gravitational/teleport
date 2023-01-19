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
  const [confirmChangeTo, setConfirmChangeTo] = useState<ClusterUri | null>(
    null
  );
  const clusters = useClusters();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, [setIsPopoverOpened]);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        'toggle-clusters': togglePopover,
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
  background: ${props => props.theme.colors.primary.light};
`;
