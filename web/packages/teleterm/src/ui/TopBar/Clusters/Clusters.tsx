import React, { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import styled from 'styled-components';
import { Box } from 'design';
import { useClusters } from './useClusters';
import { ClusterSelector } from './ClusterSelector/ClusterSelector';
import { ClustersFilterableList } from './ClustersFilterableList/ClustersFilterableList';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

export function Clusters() {
  const iconRef = useRef();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
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

  function selectItem(id: string): void {
    setIsPopoverOpened(false);
    clusters.selectItem(id);
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
  background: ${props => props.theme.colors.primary.dark};
`;
