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

import { JSX } from 'react';
import styled from 'styled-components';

import { Box, Flex, Label, P3, Text } from 'design';
import { ShieldCheck, ShieldWarning } from 'design/Icon';
import Link from 'design/Link';

import { LoggedInUser } from 'teleterm/services/tshd/types';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { DeviceTrustStatus } from 'teleterm/ui/TopBar/Identity/Identity';

import { IdentityRootCluster } from '../useIdentity';
import { AddNewClusterItem } from './AddNewClusterItem';
import { IdentityListItem } from './IdentityListItem';

export function IdentityList(props: {
  loggedInUser: LoggedInUser;
  clusters: IdentityRootCluster[];
  onSelectCluster(clusterUri: string): void;
  onAddCluster(): void;
  onLogout(clusterUri: string): void;
  deviceTrustStatus: DeviceTrustStatus;
}) {
  return (
    <Box minWidth="200px">
      {props.loggedInUser && (
        <>
          <Flex px={3} pt={2} pb={2} justifyContent="space-between">
            <Flex flexDirection="column" gap={2}>
              <Text bold>{props.loggedInUser.name}</Text>
              <Flex flexWrap="wrap" gap={1}>
                {props.loggedInUser.roles.map(role => (
                  <Label key={role} kind="secondary">
                    {role}
                  </Label>
                ))}
              </Flex>
              <DeviceTrustMessage status={props.deviceTrustStatus} />
            </Flex>
          </Flex>
          <Separator />
        </>
      )}
      <KeyboardArrowsNavigation>
        {focusGrabber}
        <Box>
          {props.clusters.map((cluster, index) => (
            <IdentityListItem
              key={cluster.uri}
              index={index}
              cluster={cluster}
              onSelect={() => props.onSelectCluster(cluster.uri)}
              onLogout={() => props.onLogout(cluster.uri)}
            />
          ))}
        </Box>
        <Separator />
        <Box>
          <AddNewClusterItem
            index={props.clusters.length + 1}
            onClick={props.onAddCluster}
          />
        </Box>
      </KeyboardArrowsNavigation>
    </Box>
export function ClusterList(props: {
  clusters: Cluster[];
  onSelect(clusterUri: RootClusterUri): void;
  onLogout?(clusterUri: RootClusterUri): void;
  onAdd(): void;
}) {
  return (
    <>
      {props.clusters.map((cluster, index) => (
        <IdentityListItem
          key={cluster.uri}
          index={index}
          cluster={cluster}
          onSelect={() => props.onSelect(cluster.uri)}
          onLogout={
            props.onLogout ? () => props.onLogout(cluster.uri) : undefined
          }
        />
      ))}
      <AddClusterItem index={props.clusters.length + 1} onClick={props.onAdd} />
    </>
  );
}

function DeviceTrustMessage(props: { status: DeviceTrustStatus }) {
  let message: JSX.Element | undefined;
  switch (props.status) {
    case 'enrolled':
      message = (
        <>
          <ShieldCheck color="success.main" size="small" mb="2px" />
          <P3>Access secured with device trust.</P3>
        </>
      );
      break;
    case 'requires-enrollment':
      message = (
        <>
          <ShieldWarning color="warning.main" size="small" mb="2px" />
          <P3>
            Full access requires a trusted device.{' '}
            <Link
              href="https://goteleport.com/docs/admin-guides/access-controls/device-trust/guide/#step-22-enroll-device"
              target="_blank"
            >
              Learn how to enroll your device.
            </Link>
          </P3>
        </>
      );
      break;
    case 'none':
      break;
    default:
      props.status satisfies never;
  }

  if (message) {
    return (
      <Flex gap={1} color="text.slightlyMuted">
        {message}
      </Flex>
    );
  }
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

const Separator = styled.div`
  background: ${props => props.theme.colors.spotBackground[1]};
  height: 1px;
`;
