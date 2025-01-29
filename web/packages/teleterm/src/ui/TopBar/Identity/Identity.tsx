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
import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

import * as tshd from 'teleterm/services/tshd/types';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import {
  useKeyboardShortcutFormatters,
  useKeyboardShortcuts,
} from 'teleterm/ui/services/keyboardShortcuts';

import { ActiveCluster, ClusterList } from './IdentityList/IdentityList';
import { IdentitySelector } from './IdentitySelector/IdentitySelector';
import { useIdentity } from './useIdentity';

export function IdentityContainer() {
  const {
    activeRootCluster,
    rootClusters,
    changeRootCluster,
    logout,
    addCluster,
    refreshCluster,
    changeColor,
  } = useIdentity();
  const selectorRef = useRef<HTMLButtonElement>();
  const [open, setOpen] = useState(false);
  const { getLabelWithAccelerator } = useKeyboardShortcutFormatters();
  const togglePopover = useCallback(() => {
    setOpen(wasOpened => !wasOpened);
  }, []);

  useKeyboardShortcuts(
    useMemo(
      () => ({
        openProfiles: togglePopover,
      }),
      [togglePopover]
    )
  );

  const makeTitle = (userWithClusterName: string | undefined) =>
    getLabelWithAccelerator(
      [userWithClusterName, 'Open Profiles'].filter(Boolean).join('\n'),
      'openProfiles'
    );

  function withClose<T extends (...args: any[]) => any>(
    fn: T
  ): (...args: Parameters<T>) => ReturnType<T> {
    return (...args) => {
      setOpen(false);
      return fn(...args);
    };
  }

  const deviceTrustStatus = calculateDeviceTrustStatus(
    activeRootCluster?.loggedInUser
  );
  const activeColor = useStoreSelector(
    'workspacesService',
    useCallback(
      state =>
        activeRootCluster
          ? state.workspaces[activeRootCluster.uri]?.color
          : undefined,
      [activeRootCluster]
    )
  );

  return (
    <>
      <IdentitySelector
        ref={selectorRef}
        onClick={togglePopover}
        open={open}
        activeCluster={activeRootCluster}
        activeColor={activeColor}
        makeTitle={makeTitle}
        deviceTrustStatus={deviceTrustStatus}
      />
      <Popover
        open={open}
        anchorEl={selectorRef.current}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        onClose={() => setOpen(false)}
        popoverCss={() => `max-width: min(450px, 90%)`}
      >
        <Container>
          {activeRootCluster && (
            <ActiveCluster
              activeCluster={activeRootCluster}
              activeColor={activeColor}
              onChangeColor={changeColor}
              onLogout={withClose(() => logout(activeRootCluster.uri))}
              onRefresh={withClose(() => refreshCluster(activeRootCluster.uri))}
              deviceTrustStatus={deviceTrustStatus}
            />
          )}
          <KeyboardArrowsNavigation>
            {focusGrabber}
            <ClusterList
              clusters={rootClusters}
              onSelect={withClose(changeRootCluster)}
              onLogout={withClose(logout)}
              onAdd={withClose(addCluster)}
            />
          </KeyboardArrowsNavigation>
        </Container>
      </Popover>
    </>
  );
}

const Container = styled(Box)`
  background: ${props => props.theme.colors.levels.elevated};
  min-width: 300px;
  width: 100%;
`;

export type DeviceTrustStatus = 'none' | 'enrolled' | 'requires-enrollment';

function calculateDeviceTrustStatus(
  loggedInUser: tshd.LoggedInUser
): DeviceTrustStatus {
  if (!loggedInUser) {
    return 'none';
  }
  if (loggedInUser.isDeviceTrusted) {
    return 'enrolled';
  }
  if (
    loggedInUser.trustedDeviceRequirement === TrustedDeviceRequirement.REQUIRED
  ) {
    return 'requires-enrollment';
  }
  return 'none';
}

// Hack - for some reason xterm.js doesn't allow moving focus to the Identity popover
// when it is focused using element.focus().
// It used to restore focus after the popover was closed, but this no longer seems to work.
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
