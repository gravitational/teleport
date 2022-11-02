import React, {
  useCallback,
  useMemo,
  useRef,
  useState,
  useImperativeHandle,
} from 'react';
import styled from 'styled-components';
import { Box } from 'design';
import Popover from 'design/Popover';

import {
  useKeyboardShortcuts,
  useKeyboardShortcutFormatters,
} from 'teleterm/ui/services/keyboardShortcuts';

import * as tshd from 'teleterm/services/tshd/types';

import { IdentityRootCluster, useIdentity } from './useIdentity';
import { IdentityList } from './IdentityList/IdentityList';
import { IdentitySelector } from './IdentitySelector/IdentitySelector';
import { EmptyIdentityList } from './EmptyIdentityList/EmptyIdentityList';

export function IdentityContainer() {
  const {
    activeRootCluster,
    rootClusters,
    changeRootCluster,
    logout,
    addCluster,
  } = useIdentity();
  const { getLabelWithShortcut } = useKeyboardShortcutFormatters();

  const presenterRef = useRef<IdentityHandler>();

  useKeyboardShortcuts(
    useMemo(
      () => ({
        'toggle-identity': presenterRef.current?.togglePopover,
      }),
      [presenterRef.current?.togglePopover]
    )
  );

  const makeTitle = (userWithClusterName: string | undefined) =>
    getLabelWithShortcut(
      [userWithClusterName, 'Open Profiles'].filter(Boolean).join('\n'),
      'toggle-identity'
    );

  return (
    <Identity
      ref={presenterRef}
      activeRootCluster={activeRootCluster}
      rootClusters={rootClusters}
      changeRootCluster={changeRootCluster}
      logout={logout}
      addCluster={addCluster}
      makeTitle={makeTitle}
    />
  );
}

export type IdentityHandler = { togglePopover: () => void };

export type IdentityProps = {
  activeRootCluster: tshd.Cluster | undefined;
  rootClusters: IdentityRootCluster[];
  changeRootCluster: (clusterUri: string) => Promise<void>;
  logout: (clusterUri: string) => void;
  addCluster: () => void;
  makeTitle: (userWithClusterName: string | undefined) => string;
};

export const Identity = React.forwardRef<IdentityHandler, IdentityProps>(
  (
    {
      activeRootCluster,
      rootClusters,
      changeRootCluster,
      logout,
      addCluster,
      makeTitle,
    },
    ref
  ) => {
    const selectorRef = useRef<HTMLButtonElement>();
    const [isPopoverOpened, setIsPopoverOpened] = useState(false);

    const togglePopover = useCallback(() => {
      setIsPopoverOpened(wasOpened => !wasOpened);
    }, [setIsPopoverOpened]);

    function withClose<T extends (...args) => any>(
      fn: T
    ): (...args: Parameters<T>) => ReturnType<T> {
      return (...args) => {
        setIsPopoverOpened(false);
        return fn(...args);
      };
    }

    useImperativeHandle(ref, () => ({
      togglePopover: () => {
        togglePopover();
      },
    }));

    const loggedInUser = activeRootCluster?.loggedInUser;

    return (
      <>
        <IdentitySelector
          ref={selectorRef}
          onClick={togglePopover}
          isOpened={isPopoverOpened}
          userName={loggedInUser?.name}
          clusterName={activeRootCluster?.name}
          makeTitle={makeTitle}
        />
        <Popover
          open={isPopoverOpened}
          anchorEl={selectorRef.current}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          transformOrigin={{ vertical: 'top', horizontal: 'right' }}
          onClose={() => setIsPopoverOpened(false)}
          popoverCss={() => `max-width: min(560px, 90%)`}
        >
          <Container>
            {rootClusters.length ? (
              <IdentityList
                loggedInUser={loggedInUser}
                clusters={rootClusters}
                onSelectCluster={withClose(changeRootCluster)}
                onLogout={withClose(logout)}
                onAddCluster={withClose(addCluster)}
              />
            ) : (
              <EmptyIdentityList onConnect={withClose(addCluster)} />
            )}
          </Container>
        </Popover>
      </>
    );
  }
);

const Container = styled(Box)`
  background: ${props => props.theme.colors.primary.light};
`;
