import React, { useCallback, useMemo, useRef, useState } from 'react';
import Popover from 'design/Popover';
import { useIdentity } from './useIdentity';
import { IdentityList } from './IdentityList/IdentityList';
import { IdentitySelector } from './IdentitySelector/IdentitySelector';
import { useKeyboardShortcuts } from 'teleterm/ui/services/keyboardShortcuts';

export function Identity() {
  const selectorRef = useRef<HTMLButtonElement>();
  const [isPopoverOpened, setIsPopoverOpened] = useState(false);
  const {
    activeRootCluster,
    rootClusters,
    changeRootCluster,
    logout,
    removeCluster,
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
        hostName={activeRootCluster?.name}
      />
      <Popover
        open={isPopoverOpened}
        anchorEl={selectorRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        onClose={() => setIsPopoverOpened(false)}
      >
        {focusGrabber}
        <IdentityList
          loggedInUser={loggedInUser}
          clusters={rootClusters}
          selectCluster={changeRootCluster}
          logout={logout}
          removeCluster={removeCluster}
          addCluster={addCluster}
        />
      </Popover>
    </>
  );
}

// Hack - for some reason xterm.js doesn't allow moving a focus to the Identity popover
// when it is focused using element.focus(). Moreover, it looks like this solution has a benefit
// of returning the focus to the previously focused element when popover is closed.
const focusGrabber = (
  <input
    style={{
      opacity: 0,
      position: 'absolute',
      height: 0,
      zIndex: -1,
    }}
    autoFocus={true}
  />
);
