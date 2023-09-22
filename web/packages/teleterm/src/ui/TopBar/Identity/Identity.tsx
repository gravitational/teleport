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
  const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();

  const presenterRef = useRef<IdentityHandler>();

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openProfiles: presenterRef.current?.togglePopover,
      }),
      [presenterRef.current?.togglePopover]
    )
  );

  const makeTitle = (userWithClusterName: string | undefined) =>
    getLabelWithAccelerator(
      [userWithClusterName, 'Open Profiles'].filter(Boolean).join('\n'),
      'openProfiles'
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
  background: ${props => props.theme.colors.levels.surface};
`;
