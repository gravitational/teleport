import React, { useCallback, useMemo, useRef, useState } from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import Popover from 'design/Popover';
import { useIdentity } from './useIdentity';
import { IdentityList } from './IdentityList/IdentityList';
import { IdentitySelector } from './IdentitySelector/IdentitySelector';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';
import { EmptyIdentityList } from './EmptyIdentityList/EmptyIdentityList';

export function Identity() {
  const selectorRef = useRef<HTMLButtonElement>();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const {
    activeRootCluster,
    rootClusters,
    changeRootCluster,
    logout,
    addCluster,
  } = useIdentity();

  const togglePopover = useCallback(() => {
    setIsPopoverOpened(wasOpened => !wasOpened);
  }, [setIsPopoverOpened]);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        'toggle-identity': togglePopover,
      }),
      [togglePopover]
    )
  );

  const loggedInUser = activeRootCluster?.loggedInUser;
  return (
    <>
      <IdentitySelector
        ref={selectorRef}
        onClick={togglePopover}
        isOpened={isPopoverOpened}
        userName={loggedInUser?.name}
        clusterName={activeRootCluster?.name}
      />
      <Popover
        open={isPopoverOpened}
        anchorEl={selectorRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        <Container>
          {rootClusters.length ? (
            <IdentityList
              loggedInUser={loggedInUser}
              clusters={rootClusters}
              onSelectCluster={changeRootCluster}
              onLogout={logout}
              onAddCluster={addCluster}
            />
          ) : (
            <EmptyIdentityList />
          )}
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.primary.dark};
`;
