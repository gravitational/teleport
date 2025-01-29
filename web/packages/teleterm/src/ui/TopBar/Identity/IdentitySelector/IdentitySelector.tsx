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

import { forwardRef } from 'react';

import { Box } from 'design';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { ProfileColor } from 'teleterm/ui/services/workspacesService';
import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';
import { DeviceTrustStatus } from 'teleterm/ui/TopBar/Identity/Identity';
import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';
import { getUserWithClusterName } from 'teleterm/ui/utils';

import { getClusterLetter } from '../IdentityList/IdentityListItem';
import { UserIcon } from './UserIcon';

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  {
    open: boolean;
    activeCluster: Cluster | undefined;
    onClick(): void;
    makeTitle(userWithClusterName: string | undefined): string;
    deviceTrustStatus: DeviceTrustStatus;
    activeColor: ProfileColor;
  }
>((props, ref) => {
  const selectorText =
    props.activeCluster &&
    getUserWithClusterName({
      clusterName: props.activeCluster.name,
      userName: props.activeCluster.loggedInUser?.name,
    });
  const title = props.makeTitle(selectorText);

  return (
    <TopBarButton
      isOpened={props.open}
      ref={ref}
      onClick={props.onClick}
      title={title}
      css={`
        position: relative;
      `}
    >
      {props.activeCluster ? (
        <Box>
          <UserIcon
            color={props.activeColor}
            letter={getClusterLetter(props.activeCluster)}
          />
          {props.deviceTrustStatus === 'requires-enrollment' && (
            <ConnectionStatusIndicator
              status={'warning'}
              css={`
                position: absolute;
                top: 1.5px;
                right: 1.5px;
              `}
            />
          )}
        </Box>
      ) : (
        <UserIcon letter="+" />
      )}
    </TopBarButton>
  );
});
